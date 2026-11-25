package main

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := jpeg.Encode(&out, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func designServer(t *testing.T) (*designStore, *http.ServeMux) {
	t.Helper()
	store, err := newDesignStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	store.routes(mux)
	return store, mux
}

func do(mux *http.ServeMux, method, target, contentType string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, bytes.NewReader(body))
	r = withAccount(r, account{Name: "tester", Role: "admin"})
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func connectInstagram(t *testing.T) {
	t.Helper()
	t.Setenv("ARECOSYS_PUBLIC_BASE_URL", "https://dash.example.test")
	t.Setenv("ARECOSYS_INSTAGRAM_USER_ID", "123")
	t.Setenv("ARECOSYS_INSTAGRAM_ACCESS_TOKEN", "token")
}

func TestUploadValidation(t *testing.T) {
	_, mux := designServer(t)
	if w := do(mux, "POST", "/api/design/assets?folder=Brand", "text/plain", []byte("x")); w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("wrong type: %d", w.Code)
	}
	if w := do(mux, "POST", "/api/design/assets?folder=Brand", "image/png", jpegBytes(t)); w.Code != http.StatusBadRequest {
		t.Errorf("mismatched content: %d", w.Code)
	}
	if w := do(mux, "POST", "/api/design/assets?folder=Nope", "image/jpeg", jpegBytes(t)); w.Code != http.StatusBadRequest {
		t.Errorf("unknown folder: %d", w.Code)
	}
	if w := do(mux, "POST", "/api/design/assets?folder=Brand&name=Logo", "image/jpeg", jpegBytes(t)); w.Code != http.StatusOK {
		t.Errorf("valid upload: %d", w.Code)
	}
}

func TestPostWorkflow(t *testing.T) {
	store, mux := designServer(t)
	upload := do(mux, "POST", "/api/design/assets?folder=Posts&name=Post", "image/jpeg", jpegBytes(t))
	id := store.data.Assets[0].ID
	if upload.Code != http.StatusOK {
		t.Fatalf("upload: %d", upload.Code)
	}
	if w := do(mux, "POST", "/api/design/posts", "application/json", []byte(`{"caption":"`+strings.Repeat("#a ", 31)+`","asset":"`+id+`"}`)); w.Code != http.StatusBadRequest {
		t.Errorf("too many hashtags: %d", w.Code)
	}
	if w := do(mux, "POST", "/api/design/posts", "application/json", []byte(`{"caption":"Hello","asset":"missing"}`)); w.Code != http.StatusBadRequest {
		t.Errorf("missing image: %d", w.Code)
	}
	if w := do(mux, "POST", "/api/design/posts", "application/json", []byte(`{"caption":"Hello","asset":"`+id+`"}`)); w.Code != http.StatusOK {
		t.Fatalf("create post: %d", w.Code)
	}
	postID := store.data.Posts[0].ID
	patch := func(body string) int {
		return do(mux, "PATCH", "/api/design/posts/"+postID, "application/json", []byte(body)).Code
	}
	if c := patch(`{"action":"publish"}`); c != http.StatusConflict {
		t.Errorf("publish before approval: %d", c)
	}
	if c := patch(`{"action":"approve"}`); c != http.StatusOK {
		t.Errorf("approve: %d", c)
	}
	if c := patch(`{"caption":"edited"}`); c != http.StatusConflict {
		t.Errorf("caption edit after approval: %d", c)
	}
	if c := patch(`{"action":"schedule","scheduledAt":"2999-01-01T00:00:00Z"}`); c != http.StatusConflict {
		t.Errorf("schedule without Instagram: %d", c)
	}
	connectInstagram(t)
	soon := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	if c := patch(`{"action":"schedule","scheduledAt":"2999-01-01T00:00:00Z"}`); c != http.StatusBadRequest {
		t.Errorf("schedule too far ahead: %d", c)
	}
	if c := patch(`{"action":"schedule","scheduledAt":"` + soon + `"}`); c != http.StatusOK || store.data.Posts[0].Status != "Scheduled" {
		t.Errorf("schedule: %d %s", c, store.data.Posts[0].Status)
	}
	if w := do(mux, "DELETE", "/api/design/assets/"+id, "", nil); w.Code != http.StatusConflict {
		t.Errorf("delete image used by queued post: %d", w.Code)
	}
	if c := patch(`{"action":"unapprove"}`); c != http.StatusOK || store.data.Posts[0].Status != "Draft" {
		t.Errorf("unapprove: %d", c)
	}
}

func TestPublicMediaNeedsSignatureAndPublishingPost(t *testing.T) {
	store, mux := designServer(t)
	do(mux, "POST", "/api/design/assets?folder=Posts", "image/jpeg", jpegBytes(t))
	id := store.data.Assets[0].ID
	do(mux, "POST", "/api/design/posts", "application/json", []byte(`{"caption":"Hi","asset":"`+id+`"}`))
	fetch := func(target string) int {
		w := httptest.NewRecorder()
		store.public(w, httptest.NewRequest("GET", target, nil))
		return w.Code
	}
	connectInstagram(t)
	signed := store.signedURL(id)
	target := strings.TrimPrefix(signed, "https://dash.example.test")
	if c := fetch(target); c != http.StatusNotFound {
		t.Errorf("draft post image must not be public: %d", c)
	}
	store.data.Posts[0].Status = "Publishing"
	if c := fetch(target); c != http.StatusOK {
		t.Errorf("publishing post image: %d", c)
	}
	if c := fetch(strings.Replace(target, "sig=", "sig=0", 1)); c != http.StatusNotFound {
		t.Errorf("bad signature: %d", c)
	}
}

func TestInterruptedPublishIsNotRetried(t *testing.T) {
	dir := t.TempDir()
	store, _ := newDesignStore(dir)
	store.data.Posts = []post{{ID: "p1", Status: "Publishing"}}
	if err := store.save(store.clone()); err != nil {
		t.Fatal(err)
	}
	reopened, err := newDesignStore(dir)
	if err != nil || reopened.data.Posts[0].Status != "Failed" {
		t.Fatalf("status after restart: %v %v", err, reopened.data.Posts)
	}
}

func TestInstagramPublishFlow(t *testing.T) {
	var calls []string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch {
		case r.URL.Path == "/123/media":
			r.ParseForm()
			if r.PostForm.Get("image_url") != "https://img.example.test/a.jpg" || r.PostForm.Get("access_token") != "token" {
				http.Error(w, `{"error":{"message":"bad form"}}`, 400)
				return
			}
			w.Write([]byte(`{"id":"555"}`))
		case r.URL.Path == "/555":
			w.Write([]byte(`{"status_code":"FINISHED"}`))
		case r.URL.Path == "/123/media_publish":
			w.Write([]byte(`{"id":"999"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()
	old := igGraph
	igGraph = fake.URL + "/"
	defer func() { igGraph = old }()
	connectInstagram(t)
	id, err := instagramPublish(context.Background(), []string{"https://img.example.test/a.jpg"}, "caption")
	if err != nil || id != "999" {
		t.Fatalf("publish: %q %v", id, err)
	}
	if len(calls) != 3 {
		t.Errorf("calls: %v", calls)
	}
}

func TestCarouselPostAndPublish(t *testing.T) {
	store, mux := designServer(t)
	for i := 0; i < 2; i++ {
		do(mux, "POST", "/api/design/assets?folder=Posts", "image/jpeg", jpegBytes(t))
	}
	ids := []string{store.data.Assets[0].ID, store.data.Assets[1].ID}
	body := `{"caption":"Swipe","assets":["` + ids[0] + `","` + ids[1] + `"]}`
	if w := do(mux, "POST", "/api/design/posts", "application/json", []byte(body)); w.Code != http.StatusOK {
		t.Fatalf("create carousel: %d", w.Code)
	}
	if p := store.data.Posts[0]; p.Asset != ids[0] || len(p.Assets) != 2 {
		t.Errorf("carousel not stored: %+v", p)
	}
	if w := do(mux, "POST", "/api/design/posts", "application/json", []byte(`{"caption":"x","assets":["`+ids[0]+`","nope"]}`)); w.Code != http.StatusBadRequest {
		t.Errorf("unknown carousel image: %d", w.Code)
	}
	if w := do(mux, "DELETE", "/api/design/assets/"+ids[1], "", nil); w.Code != http.StatusConflict {
		t.Errorf("delete carousel image in queue: %d", w.Code)
	}
	store.data.Posts[0].Status = "Publishing"
	connectInstagram(t)
	w := httptest.NewRecorder()
	store.public(w, httptest.NewRequest("GET", strings.TrimPrefix(store.signedURL(ids[1]), "https://dash.example.test"), nil))
	if w.Code != http.StatusOK {
		t.Errorf("second slide must be public while publishing: %d", w.Code)
	}

	var forms []string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		forms = append(forms, r.URL.Path+" "+r.PostForm.Get("media_type")+" "+r.PostForm.Get("children"))
		switch {
		case r.URL.Path == "/123/media":
			if r.PostForm.Get("media_type") == "CAROUSEL" {
				w.Write([]byte(`{"id":"300"}`))
			} else if len(forms) == 1 {
				w.Write([]byte(`{"id":"101"}`))
			} else {
				w.Write([]byte(`{"id":"102"}`))
			}
		case strings.HasPrefix(r.URL.Path, "/10") || r.URL.Path == "/300":
			w.Write([]byte(`{"status_code":"FINISHED"}`))
		case r.URL.Path == "/123/media_publish":
			w.Write([]byte(`{"id":"999"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()
	old := igGraph
	igGraph = fake.URL + "/"
	defer func() { igGraph = old }()
	id, err := instagramPublish(context.Background(), []string{"https://i.example.test/1.jpg", "https://i.example.test/2.jpg"}, "Swipe")
	if err != nil || id != "999" {
		t.Fatalf("carousel publish: %q %v", id, err)
	}
	found := false
	for _, form := range forms {
		if form == "/123/media CAROUSEL 101,102" {
			found = true
		}
	}
	if !found {
		t.Errorf("carousel container not created with both children: %v", forms)
	}
}
