package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
)

// maxResponseBytes bounds how much of a response body is buffered. The GitHub
// API over TLS is trusted and paginates, so this is defense-in-depth against a
// misconfigured/compromised endpoint, set well above any real API page.
const maxResponseBytes = 64 << 20 // 64 MiB

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Get(ctx context.Context, path string, out any) error {
	body, _, err := c.do(ctx, path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// Post sends a JSON body to path and, when out is non-nil, decodes the response
// into it. GitHub returns the created review/comment JSON; callers that do not
// need it pass out=nil.
func (c *Client) Post(ctx context.Context, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("github: encoding request body: %w", err)
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

// maxPaginatedPages bounds how many pages GetPaginated will fetch; a real PR
// never comes close, so hitting it indicates a broken or malicious endpoint.
const maxPaginatedPages = 1000

func (c *Client) GetPaginated(ctx context.Context, path string, out any) error {
	target := reflect.ValueOf(out)
	if target.Kind() != reflect.Pointer || target.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("github: paginated output must be pointer to slice")
	}
	sliceValue := target.Elem()
	// visited defends against a server/proxy returning rel="next" links that
	// cycle (self-loops as well as longer A→B→A cycles), which would loop
	// forever.
	visited := make(map[string]struct{})
	nextPath := path
	for nextPath != "" {
		if _, seen := visited[nextPath]; seen {
			break
		}
		if len(visited) >= maxPaginatedPages {
			return fmt.Errorf("github: pagination for %s exceeded %d pages", path, maxPaginatedPages)
		}
		visited[nextPath] = struct{}{}
		body, resp, err := c.do(ctx, nextPath)
		if err != nil {
			return err
		}
		page := reflect.New(sliceValue.Type())
		if err := json.Unmarshal(body, page.Interface()); err != nil {
			return err
		}
		sliceValue.Set(reflect.AppendSlice(sliceValue, page.Elem()))
		nextPath = nextLink(resp.Header.Get("Link"))
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
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
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
	return respBody, resp, nil
}

// APIError is returned when the GitHub API responds with a >= 300 status. It
// carries the HTTP status so callers (e.g. the review publisher) can branch on
// specific codes such as 422 (comment line not part of the diff) via errors.As.
type APIError struct {
	Method string
	URL    string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	message := fmt.Sprintf("github: %s %s: status %d", e.Method, e.URL, e.Status)
	if text := strings.TrimSpace(e.Body); text != "" {
		message += ": " + text
	}
	if e.Status == http.StatusNotFound {
		message += " (check --repo, --id, and token repo access)"
	}
	return message
}

func newAPIError(method, requestURL string, status int, body []byte) *APIError {
	return &APIError{Method: method, URL: requestURL, Status: status, Body: string(body)}
}

// IsUnprocessable reports whether err is a 422 from the GitHub API — used to
// detect a review comment whose line is not part of the diff so the caller can
// fall back rather than dropping the finding.
func IsUnprocessable(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusUnprocessableEntity
}

func escapeRepo(repo string) string {
	parts := strings.Split(repo, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func nextLink(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitSeq(header, ",")
	for part := range parts {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start < 0 || end < 0 || end <= start+1 {
			continue
		}
		link := part[start+1 : end]
		parsed, err := url.Parse(link)
		if err != nil {
			return ""
		}
		return parsed.RequestURI()
	}
	return ""
}
