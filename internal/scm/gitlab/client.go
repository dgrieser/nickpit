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

// Post sends a JSON body to path and, when out is non-nil, decodes the response
// into it. GitLab returns the created note/discussion JSON; callers that do not
// need it pass out=nil.
func (c *Client) Post(ctx context.Context, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("gitlab: encoding request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	respBody, _, err := c.doRequest(ctx, http.MethodPost, path, reader, "application/json")
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(respBody, out)
}

// Delete issues a DELETE request (GitLab answers 204 on success).
func (c *Client) Delete(ctx context.Context, path string) error {
	_, _, err := c.doRequest(ctx, http.MethodDelete, path, nil, "")
	return err
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
	return c.doRequest(ctx, http.MethodGet, path, nil, "")
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, *http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, nil, err
	}
	if c.token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, nil, newAPIError(method, req.URL.String(), resp.StatusCode, errBody)
	}
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, nil, err
	}
	if !looksLikeJSON(resp.Header.Get("Content-Type"), respBody) {
		return nil, nil, gitLabNonJSONError(method, req.URL.String(), resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
	}
	return respBody, resp, nil
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

func gitLabNonJSONError(method, requestURL string, status int, contentType string, body []byte) error {
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 200 {
		snippet = snippet[:200] + "…"
	}
	ct := contentType
	if ct == "" {
		ct = "<empty>"
	}
	return fmt.Errorf(
		"gitlab: %s %s: status %d returned non-JSON body (content-type=%s): %s "+
			"(check --gitlab-base-url; it must point at the GitLab API root, e.g. https://gitlab.example.com/api/v4)",
		method, requestURL, status, ct, snippet,
	)
}

func escapeProject(project string) string {
	return url.PathEscape(project)
}

// APIError is returned when the GitLab API responds with a >= 300 status. It
// carries the HTTP status so callers (e.g. the review publisher) can branch on
// specific codes such as 422 (position not in diff) via errors.As.
type APIError struct {
	Method string
	URL    string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	message := fmt.Sprintf("gitlab: %s %s: status %d", e.Method, e.URL, e.Status)
	if text := strings.TrimSpace(e.Body); text != "" {
		message += ": " + text
	}
	if e.Status == http.StatusNotFound {
		message += " (check --repo, --id, --gitlab-base-url, and token project access)"
	}
	return message
}

func newAPIError(method, requestURL string, status int, body []byte) *APIError {
	return &APIError{Method: method, URL: requestURL, Status: status, Body: string(body)}
}

// withPage appends pagination parameters. per_page is always maximized (100,
// GitLab's cap): the default of 20 turns a busy MR's note/discussion listing
// into 5x the requests.
func withPage(path string, page int) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	if page <= 1 {
		return fmt.Sprintf("%s%sper_page=100", path, separator)
	}
	return fmt.Sprintf("%s%sper_page=100&page=%d", path, separator, page)
}
