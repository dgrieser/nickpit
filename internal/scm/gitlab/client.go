package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://gitlab.com/api/v4"

// maxResponseBytes bounds how much of a success response body is buffered. The
// GitLab API over TLS is trusted and paginates, so this is defense-in-depth set
// well above any real API page.
const maxResponseBytes = 64 << 20 // 64 MiB

var apiVersionPathRegex = regexp.MustCompile(`/api/v\d+(/|$)`)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: NormalizeBaseURL(baseURL),
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NormalizeBaseURL canonicalizes a user-supplied GitLab base URL: prepends
// https:// when no scheme is present and appends /api/v4 when no API version
// segment is in the path. Empty input returns the gitlab.com default.
func NormalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultBaseURL
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	raw = strings.TrimRight(raw, "/")
	if !apiVersionPathRegex.MatchString(raw) {
		raw += "/api/v4"
	}
	return raw
}

func (c *Client) Get(ctx context.Context, path string, out any) error {
	body, _, err := c.do(ctx, path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func (c *Client) GetPaginated(ctx context.Context, path string, out any) error {
	target := reflect.ValueOf(out)
	if target.Kind() != reflect.Pointer || target.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("gitlab: paginated output must be pointer to slice")
	}
	sliceValue := target.Elem()
	nextPath := path
	page := 1
	for nextPath != "" {
		body, resp, err := c.do(ctx, withPage(nextPath, page))
		if err != nil {
			return err
		}
		tmp := reflect.New(sliceValue.Type())
		if err := json.Unmarshal(body, tmp.Interface()); err != nil {
			return err
		}
		sliceValue.Set(reflect.AppendSlice(sliceValue, tmp.Elem()))
		next := resp.Header.Get("X-Next-Page")
		if next == "" {
			nextPath = ""
			continue
		}
		nextPage, err := strconv.Atoi(next)
		if err != nil || nextPage == 0 {
			nextPath = ""
			continue
		}
		page = nextPage
	}
	return nil
}

func (c *Client) do(ctx context.Context, path string) ([]byte, *http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, nil, err
	}
	if c.token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, nil, gitLabStatusError(req.URL.String(), resp.StatusCode, body)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, nil, err
	}
	if !looksLikeJSON(resp.Header.Get("Content-Type"), body) {
		return nil, nil, gitLabNonJSONError(req.URL.String(), resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
	return body, resp, nil
}

func looksLikeJSON(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "json") {
		return true
	}
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	if len(trimmed) == 0 {
		return true
	}
	return trimmed[0] == '{' || trimmed[0] == '['
}

func gitLabNonJSONError(requestURL string, status int, contentType string, body []byte) error {
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 200 {
		snippet = snippet[:200] + "…"
	}
	ct := contentType
	if ct == "" {
		ct = "<empty>"
	}
	return fmt.Errorf(
		"gitlab: GET %s: status %d returned non-JSON body (content-type=%s): %s "+
			"(check --gitlab-base-url; it must point at the GitLab API root, e.g. https://gitlab.example.com/api/v4)",
		requestURL, status, ct, snippet,
	)
}

func escapeProject(project string) string {
	return url.PathEscape(project)
}

func gitLabStatusError(requestURL string, status int, body []byte) error {
	message := fmt.Sprintf("gitlab: GET %s: status %d", requestURL, status)
	if text := strings.TrimSpace(string(body)); text != "" {
		message += ": " + text
	}
	if status == http.StatusNotFound {
		message += " (check --repo, --id, --gitlab-base-url, and token project access)"
	}
	return fmt.Errorf("%s", message)
}

func withPage(path string, page int) string {
	if page <= 1 {
		return path
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%spage=%d", path, separator, page)
}
