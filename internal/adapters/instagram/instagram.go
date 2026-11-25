package instagram

import (
	"arecosys-dashboard/internal/platform/httpx"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Graph is a variable so tests can point it at a local fake.
var Graph = "https://graph.instagram.com/v23.0/"

var Numeric = regexp.MustCompile(`^[0-9]{1,40}$`)

// Connected reports whether publishing is configured. Instagram fetches the
// image from ARECOSYS_PUBLIC_BASE_URL, so that must be a public HTTPS address.
func Connected() bool {
	base, err := url.Parse(os.Getenv("ARECOSYS_PUBLIC_BASE_URL"))
	return err == nil && base.Scheme == "https" && base.Host != "" &&
		Numeric.MatchString(os.Getenv("ARECOSYS_INSTAGRAM_USER_ID")) &&
		os.Getenv("ARECOSYS_INSTAGRAM_ACCESS_TOKEN") != ""
}

// Call never returns the underlying transport error: it embeds the request URL,
// which carries the access token for GET requests.
func Call(ctx context.Context, method, path string, values url.Values, out any) error {
	values.Set("access_token", os.Getenv("ARECOSYS_INSTAGRAM_ACCESS_TOKEN"))
	var request *http.Request
	var err error
	if method == http.MethodGet {
		request, err = http.NewRequestWithContext(ctx, method, Graph+path+"?"+values.Encode(), nil)
	} else {
		request, err = http.NewRequestWithContext(ctx, method, Graph+path, strings.NewReader(values.Encode()))
		if err == nil {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	if err != nil {
		return errors.New("Instagram request could not be created")
	}
	response, err := httpx.Client.Do(request)
	if err != nil {
		return errors.New("Instagram could not be reached")
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<16))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var failure struct{ Error struct{ Message string } }
		_ = json.Unmarshal(body, &failure)
		message := failure.Error.Message
		if message == "" {
			message = fmt.Sprintf("status %d", response.StatusCode)
		}
		return fmt.Errorf("Instagram rejected the request: %.200s", message)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return errors.New("Instagram returned an unreadable response")
	}
	return nil
}

// igWait polls a media container until Instagram has finished fetching and processing it.
func igWait(ctx context.Context, id string) error {
	for attempt := 0; attempt < 30; attempt++ {
		var state struct {
			StatusCode string `json:"status_code"`
		}
		if err := Call(ctx, http.MethodGet, id, url.Values{"fields": {"status_code"}}, &state); err != nil {
			return err
		}
		switch state.StatusCode {
		case "FINISHED":
			return nil
		case "ERROR", "EXPIRED":
			return errors.New("Instagram could not process the image")
		}
		select {
		case <-ctx.Done():
			return errors.New("Instagram took too long to process the image")
		case <-time.After(2 * time.Second):
		}
	}
	return errors.New("Instagram took too long to process the image")
}

// Publish posts one JPEG image, or a carousel when given several. It creates the
// media container, waits for Instagram to finish with it, then publishes the container.
func Publish(ctx context.Context, imageURLs []string, caption string) (string, error) {
	if len(imageURLs) == 0 || len(imageURLs) > 10 {
		return "", errors.New("a post needs between 1 and 10 images")
	}
	user := os.Getenv("ARECOSYS_INSTAGRAM_USER_ID")
	create := func(values url.Values) (string, error) {
		var out struct {
			ID string `json:"id"`
		}
		if err := Call(ctx, http.MethodPost, user+"/media", values, &out); err != nil {
			return "", err
		}
		if !Numeric.MatchString(out.ID) {
			return "", errors.New("Instagram returned an invalid media container")
		}
		return out.ID, nil
	}
	var container string
	var err error
	if len(imageURLs) == 1 {
		container, err = create(url.Values{"image_url": {imageURLs[0]}, "caption": {caption}})
	} else {
		children := make([]string, 0, len(imageURLs))
		for _, imageURL := range imageURLs {
			child, childErr := create(url.Values{"image_url": {imageURL}, "is_carousel_item": {"true"}})
			if childErr == nil {
				childErr = igWait(ctx, child)
			}
			if childErr != nil {
				return "", childErr
			}
			children = append(children, child)
		}
		container, err = create(url.Values{"media_type": {"CAROUSEL"}, "children": {strings.Join(children, ",")}, "caption": {caption}})
	}
	if err != nil {
		return "", err
	}
	if err := igWait(ctx, container); err != nil {
		return "", err
	}
	var published struct {
		ID string `json:"id"`
	}
	if err := Call(ctx, http.MethodPost, user+"/media_publish", url.Values{"creation_id": {container}}, &published); err != nil {
		return "", err
	}
	return published.ID, nil
}
