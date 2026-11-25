package design

import (
	"arecosys-dashboard/internal/adapters/instagram"
	"arecosys-dashboard/internal/platform/httpx"
	"arecosys-dashboard/internal/security"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type asset struct {
	ID      string `json:"id"`
	Folder  string `json:"folder"`
	Name    string `json:"name"`
	Ext     string `json:"ext"`
	Created string `json:"created"`
}

type post struct {
	ID          string   `json:"id"`
	Caption     string   `json:"caption"`
	Asset       string   `json:"asset"`
	Assets      []string `json:"assets"`
	Status      string   `json:"status"`
	ScheduledAt string   `json:"scheduledAt"`
	PublishedAt string   `json:"publishedAt"`
	MediaID     string   `json:"mediaId"`
	Error       string   `json:"error"`
	Created     string   `json:"created"`
}

// images lists the post's image assets in slide order. Posts saved before carousels
// existed only have Asset.
func (p post) images() []string {
	if len(p.Assets) > 0 {
		return p.Assets
	}
	return []string{p.Asset}
}

type designData struct {
	Folders []string `json:"folders"`
	Assets  []asset  `json:"assets"`
	Posts   []post   `json:"posts"`
}

// Store keeps the design library and the Instagram post queue in ARECOSYS_MEDIA_DIR.
// Like the lead store it is meant for a single server instance.
type Store struct {
	Mu   sync.Mutex
	Dir  string
	key  []byte
	Data designData
}

var imageExts = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}

func NewStore(dir string) (*Store, error) {
	// The signing key only needs to outlive a 15 minute link, so a fresh one per start is fine.
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	store := &Store{Dir: dir, key: key, Data: designData{Folders: []string{"Brand", "Posts"}, Assets: []asset{}, Posts: []post{}}}
	if dir == "" {
		return store, nil
	}
	if err := os.MkdirAll(filepath.Join(dir, "files"), 0o700); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "design.json"))
	switch {
	case err == nil:
		if err := json.Unmarshal(raw, &store.Data); err != nil {
			return nil, errors.New("invalid design storage file")
		}
	case !os.IsNotExist(err):
		return nil, err
	}
	// A post left in Publishing by a crash may or may not have reached Instagram. Do not
	// retry it automatically, because that could post it twice.
	changed := false
	for i := range store.Data.Posts {
		if store.Data.Posts[i].Status == "Publishing" {
			store.Data.Posts[i].Status = "Failed"
			store.Data.Posts[i].Error = "Publishing was interrupted. Check Instagram before retrying."
			changed = true
		}
	}
	if changed {
		if err := store.save(store.clone()); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (d *Store) clone() designData {
	return designData{slices.Clone(d.Data.Folders), slices.Clone(d.Data.Assets), slices.Clone(d.Data.Posts)}
}

func (d *Store) save(next designData) error {
	raw, err := json.Marshal(next)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(d.Dir, ".design-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	_, writeErr := temp.Write(raw)
	if err := errors.Join(writeErr, temp.Close()); err != nil {
		return err
	}
	if err := os.Rename(temp.Name(), filepath.Join(d.Dir, "design.json")); err != nil {
		return err
	}
	d.Data = next
	return nil
}

func (d *Store) enabled(w http.ResponseWriter, r *http.Request) bool {
	if d.Dir == "" {
		http.NotFound(w, r)
		return false
	}
	return true
}

func (d *Store) findAsset(id string) (asset, bool) {
	for _, a := range d.Data.Assets {
		if a.ID == id {
			return a, true
		}
	}
	return asset{}, false
}

func (d *Store) path(a asset) string { return filepath.Join(d.Dir, "files", a.ID+a.Ext) }

func validCaption(caption string) bool {
	if len([]rune(caption)) > 2200 {
		return false
	}
	tags := 0
	for _, word := range strings.Fields(caption) {
		if strings.HasPrefix(word, "#") {
			tags++
		}
	}
	return tags <= 30
}

func (d *Store) list(w http.ResponseWriter, r *http.Request) {
	if d.Dir == "" {
		httpx.JSONResponse(w, map[string]any{"state": "demo"})
		return
	}
	d.Mu.Lock()
	data := d.clone()
	d.Mu.Unlock()
	httpx.JSONResponse(w, map[string]any{"state": "live", "data": data, "instagram": instagram.Connected()})
}

func (d *Store) addFolder(w http.ResponseWriter, r *http.Request) {
	if !d.enabled(w, r) {
		return
	}
	var item struct {
		Name string `json:"name"`
	}
	if !httpx.ReadJSON(w, r, &item) {
		return
	}
	name := strings.TrimSpace(item.Name)
	if name == "" || len([]rune(name)) > 40 {
		http.Error(w, "folder name must be 1 to 40 characters", http.StatusBadRequest)
		return
	}
	d.Mu.Lock()
	defer d.Mu.Unlock()
	for _, existing := range d.Data.Folders {
		if strings.EqualFold(existing, name) {
			http.Error(w, "folder already exists", http.StatusConflict)
			return
		}
	}
	next := d.clone()
	next.Folders = append(next.Folders, name)
	if err := d.save(next); err != nil {
		http.Error(w, "folder could not be saved", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *Store) upload(w http.ResponseWriter, r *http.Request) {
	if !d.enabled(w, r) {
		return
	}
	contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	ext, ok := imageExts[contentType]
	if !ok {
		http.Error(w, "expected a JPEG, PNG or WebP image", http.StatusUnsupportedMediaType)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
	if err != nil {
		http.Error(w, "image is larger than 8 MB", http.StatusRequestEntityTooLarge)
		return
	}
	if http.DetectContentType(body) != contentType {
		http.Error(w, "file content does not match its type", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		name = "Untitled"
	}
	if len([]rune(name)) > 120 {
		http.Error(w, "name too long", http.StatusBadRequest)
		return
	}
	folder := r.URL.Query().Get("folder")
	d.Mu.Lock()
	defer d.Mu.Unlock()
	if !slices.Contains(d.Data.Folders, folder) {
		http.Error(w, "unknown folder", http.StatusBadRequest)
		return
	}
	item := asset{ID: rand.Text(), Folder: folder, Name: name, Ext: ext, Created: time.Now().Format(time.RFC3339)}
	if err := os.WriteFile(d.path(item), body, 0o600); err != nil {
		http.Error(w, "image could not be saved", http.StatusInternalServerError)
		return
	}
	next := d.clone()
	next.Assets = append([]asset{item}, next.Assets...)
	if err := d.save(next); err != nil {
		os.Remove(d.path(item))
		http.Error(w, "image could not be saved", http.StatusInternalServerError)
		return
	}
	httpx.JSONResponse(w, item)
}

func (d *Store) serve(w http.ResponseWriter, r *http.Request) {
	if !d.enabled(w, r) {
		return
	}
	d.Mu.Lock()
	item, ok := d.findAsset(r.PathValue("id"))
	d.Mu.Unlock()
	d.sendFile(w, r, item, ok)
}

func (d *Store) sendFile(w http.ResponseWriter, r *http.Request, item asset, ok bool) {
	if !ok {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(d.path(item))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", mime.TypeByExtension(item.Ext))
	w.Header().Set("Cache-Control", "private, max-age=300")
	http.ServeContent(w, r, "", time.Time{}, file)
}

func (d *Store) deleteAsset(w http.ResponseWriter, r *http.Request) {
	if !d.enabled(w, r) {
		return
	}
	d.Mu.Lock()
	defer d.Mu.Unlock()
	item, ok := d.findAsset(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	for _, p := range d.Data.Posts {
		if slices.Contains(p.images(), item.ID) && p.Status != "Published" && p.Status != "Failed" {
			http.Error(w, "image is used by a post in the queue", http.StatusConflict)
			return
		}
	}
	next := d.clone()
	next.Assets = slices.DeleteFunc(next.Assets, func(a asset) bool { return a.ID == item.ID })
	if err := d.save(next); err != nil {
		http.Error(w, "image could not be deleted", http.StatusInternalServerError)
		return
	}
	os.Remove(d.path(item))
	w.WriteHeader(http.StatusNoContent)
}

func (d *Store) addPost(w http.ResponseWriter, r *http.Request) {
	if !d.enabled(w, r) {
		return
	}
	var item struct {
		Caption string   `json:"caption"`
		Asset   string   `json:"asset"`
		Assets  []string `json:"assets"`
	}
	if !httpx.ReadJSON(w, r, &item) {
		return
	}
	caption := strings.TrimSpace(item.Caption)
	if !validCaption(caption) {
		http.Error(w, "caption must be at most 2200 characters with at most 30 hashtags", http.StatusBadRequest)
		return
	}
	d.Mu.Lock()
	defer d.Mu.Unlock()
	ids := item.Assets
	if len(ids) == 0 && item.Asset != "" {
		ids = []string{item.Asset}
	}
	if len(ids) < 1 || len(ids) > 10 {
		http.Error(w, "a post needs between 1 and 10 images", http.StatusBadRequest)
		return
	}
	for _, id := range ids {
		if image, ok := d.findAsset(id); !ok || image.Ext != ".jpg" {
			http.Error(w, "post images must be saved JPEGs", http.StatusBadRequest)
			return
		}
	}
	created := post{ID: rand.Text(), Caption: caption, Asset: ids[0], Assets: ids, Status: "Draft", Created: time.Now().Format(time.RFC3339)}
	next := d.clone()
	next.Posts = append([]post{created}, next.Posts...)
	if err := d.save(next); err != nil {
		http.Error(w, "post could not be saved", http.StatusInternalServerError)
		return
	}
	httpx.JSONResponse(w, created)
}

func (d *Store) updatePost(w http.ResponseWriter, r *http.Request) {
	if !d.enabled(w, r) {
		return
	}
	var item struct {
		Caption     *string `json:"caption"`
		Action      string  `json:"action"`
		ScheduledAt string  `json:"scheduledAt"`
	}
	if !httpx.ReadJSON(w, r, &item) {
		return
	}
	if item.Caption == nil && item.Action == "" {
		http.Error(w, "nothing to update", http.StatusBadRequest)
		return
	}
	d.Mu.Lock()
	defer d.Mu.Unlock()
	next := d.clone()
	index := slices.IndexFunc(next.Posts, func(p post) bool { return p.ID == r.PathValue("id") })
	if index < 0 {
		http.NotFound(w, r)
		return
	}
	p := &next.Posts[index]
	refuse := func() { http.Error(w, "that action is not available for this post", http.StatusConflict) }
	if item.Caption != nil {
		caption := strings.TrimSpace(*item.Caption)
		if p.Status != "Draft" && p.Status != "Failed" {
			refuse()
			return
		}
		if !validCaption(caption) {
			http.Error(w, "caption must be at most 2200 characters with at most 30 hashtags", http.StatusBadRequest)
			return
		}
		p.Caption = caption
	}
	publishNow := false
	if (item.Action == "schedule" || item.Action == "publish") && !security.RequireAdmin(w, r) {
		return
	}
	switch item.Action {
	case "":
	case "approve":
		if p.Status != "Draft" && p.Status != "Failed" {
			refuse()
			return
		}
		p.Status, p.Error = "Approved", ""
	case "unapprove":
		if p.Status != "Approved" && p.Status != "Scheduled" {
			refuse()
			return
		}
		p.Status, p.ScheduledAt = "Draft", ""
	case "schedule", "publish":
		if p.Status != "Approved" && p.Status != "Scheduled" {
			refuse()
			return
		}
		if !instagram.Connected() {
			http.Error(w, "Instagram is not connected", http.StatusConflict)
			return
		}
		if item.Action == "publish" {
			p.Status, p.ScheduledAt, publishNow = "Publishing", "", true
			break
		}
		when, err := time.Parse(time.RFC3339, item.ScheduledAt)
		if err != nil || !when.After(time.Now()) || when.After(time.Now().AddDate(0, 0, 75)) {
			http.Error(w, "choose a time within the next 75 days", http.StatusBadRequest)
			return
		}
		p.Status, p.ScheduledAt = "Scheduled", when.UTC().Format(time.RFC3339)
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	if err := d.save(next); err != nil {
		http.Error(w, "post could not be saved", http.StatusInternalServerError)
		return
	}
	if publishNow {
		go d.publish(p.ID)
	}
	httpx.JSONResponse(w, *p)
}

func (d *Store) deletePost(w http.ResponseWriter, r *http.Request) {
	if !d.enabled(w, r) {
		return
	}
	d.Mu.Lock()
	defer d.Mu.Unlock()
	next := d.clone()
	index := slices.IndexFunc(next.Posts, func(p post) bool { return p.ID == r.PathValue("id") })
	if index < 0 {
		http.NotFound(w, r)
		return
	}
	if next.Posts[index].Status == "Publishing" {
		http.Error(w, "post is being published", http.StatusConflict)
		return
	}
	next.Posts = slices.Delete(next.Posts, index, index+1)
	if err := d.save(next); err != nil {
		http.Error(w, "post could not be deleted", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *Store) sign(id string, expires int64) string {
	mac := hmac.New(sha256.New, d.key)
	fmt.Fprintf(mac, "%s|%d", id, expires)
	return hex.EncodeToString(mac.Sum(nil))
}

func (d *Store) signedURL(id string) string {
	expires := time.Now().Add(15 * time.Minute).Unix()
	base := strings.TrimRight(os.Getenv("ARECOSYS_PUBLIC_BASE_URL"), "/")
	return fmt.Sprintf("%s/public/media/%s?exp=%d&sig=%s", base, id, expires, d.sign(id, expires))
}

// Public serves the one image Instagram needs to fetch. It needs a valid signature and
// the image must belong to a post that is publishing right now.
func (d *Store) Public(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/public/media/")
	expires, err := strconv.ParseInt(r.URL.Query().Get("exp"), 10, 64)
	signature := r.URL.Query().Get("sig")
	if d.Dir == "" || err != nil || expires < time.Now().Unix() || subtle.ConstantTimeCompare([]byte(signature), []byte(d.sign(id, expires))) != 1 {
		http.NotFound(w, r)
		return
	}
	d.Mu.Lock()
	item, ok := d.findAsset(id)
	if ok {
		ok = item.Ext == ".jpg" && slices.ContainsFunc(d.Data.Posts, func(p post) bool { return p.Status == "Publishing" && slices.Contains(p.images(), id) })
	}
	d.Mu.Unlock()
	d.sendFile(w, r, item, ok)
}

func (d *Store) publish(id string) {
	d.Mu.Lock()
	var p post
	for _, candidate := range d.Data.Posts {
		if candidate.ID == id {
			p = candidate
		}
	}
	d.Mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	var urls []string
	for _, id := range p.images() {
		urls = append(urls, d.signedURL(id))
	}
	mediaID, err := instagram.Publish(ctx, urls, p.Caption)
	d.Mu.Lock()
	defer d.Mu.Unlock()
	next := d.clone()
	for i := range next.Posts {
		if next.Posts[i].ID != id {
			continue
		}
		if err != nil {
			next.Posts[i].Status, next.Posts[i].Error = "Failed", err.Error()
		} else {
			next.Posts[i].Status, next.Posts[i].Error = "Published", ""
			next.Posts[i].PublishedAt, next.Posts[i].MediaID = time.Now().Format(time.RFC3339), mediaID
		}
	}
	if saveErr := d.save(next); saveErr != nil {
		log.Print("post status could not be saved")
		d.Data = next
	}
}

// Run publishes scheduled posts when they fall due.
func (d *Store) Run() {
	for range time.Tick(30 * time.Second) {
		for _, id := range d.claimDue() {
			go d.publish(id)
		}
	}
}

func (d *Store) claimDue() []string {
	if !instagram.Connected() {
		return nil
	}
	d.Mu.Lock()
	defer d.Mu.Unlock()
	next := d.clone()
	var due []string
	for i := range next.Posts {
		p := &next.Posts[i]
		when, err := time.Parse(time.RFC3339, p.ScheduledAt)
		if p.Status == "Scheduled" && err == nil && !when.After(time.Now()) {
			p.Status, p.ScheduledAt = "Publishing", ""
			due = append(due, p.ID)
		}
	}
	if len(due) == 0 {
		return nil
	}
	if err := d.save(next); err != nil {
		log.Print("scheduled posts could not be started")
		return nil
	}
	return due
}

func (d *Store) Routes(app *http.ServeMux) {
	app.HandleFunc("GET /api/design", d.list)
	app.HandleFunc("POST /api/design/folders", d.addFolder)
	app.HandleFunc("POST /api/design/assets", d.upload)
	app.HandleFunc("DELETE /api/design/assets/{id}", d.deleteAsset)
	app.HandleFunc("POST /api/design/posts", d.addPost)
	app.HandleFunc("PATCH /api/design/posts/{id}", d.updatePost)
	app.HandleFunc("DELETE /api/design/posts/{id}", d.deletePost)
	app.HandleFunc("GET /media/{id}", d.serve)
}
