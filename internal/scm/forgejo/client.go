// Package forgejo reviews pull requests on Forgejo (and, through the shared
// API, Gitea) instances: Codeberg and self-hosted servers. The REST API lives
// under /api/v1 and follows GitHub's shape closely, with three differences
// this package absorbs: the token travels as "Authorization: token", pages are
// addressed by page/limit, and the pull request files listing carries no
// patch — the diff is a separate download.
package forgejo

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
	"regexp"
	"strings"
	"time"
)

// maxResponseBytes bounds how much of a response body is buffered. The API
// over TLS is trusted and paginates, so this is defense-in-depth against a
// misconfigured/compromised endpoint, set well above any real API page or
// pull request diff.
const maxResponseBytes = 64 << 20 // 64 MiB

// maxPageSize is the page size asked for. Forgejo caps a page at the server's
// MAX_RESPONSE_ITEMS (50 by default) and silently trims a larger request, so
// asking for more gains nothing.
const maxPageSize = 50

var apiVersionPathRegex = regexp.MustCompile(`/api/v\d+(/|$)`)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient builds a client for one instance. baseURL names the instance and
// is normalized (see NormalizeBaseURL); there is no default, because every
// Forgejo is somebody's own server.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: NormalizeBaseURL(baseURL),
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NormalizeBaseURL canonicalizes a user-supplied base URL: prepends https://
// when no scheme is present and appends /api/v1 when no API version segment
// is in the path. Empty input stays empty — an unconfigured instance is an
// error the caller words, not a default host.
func NormalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	raw = strings.TrimRight(raw, "/")
	if !apiVersionPathRegex.MatchString(raw) {
		raw += "/api/v1"
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

// GetRaw returns the response body as is, for the endpoints that answer with
// text rather than JSON: the pull request diff and raw file contents.
func (c *Client) GetRaw(ctx context.Context, path string) ([]byte, error) {
	body, _, err := c.do(ctx, path)
	return body, err
}

// Post sends a JSON body to path and, when out is non-nil, decodes the response
// into it. Forgejo returns the created review/comment JSON; callers that do not
// need it pass out=nil.
func (c *Client) Post(ctx context.Context, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("forgejo: encoding request body: %w", err)
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

// Delete issues a DELETE request (Forgejo answers 204 on success).
func (c *Client) Delete(ctx context.Context, path string) error {
	_, _, err := c.doRequest(ctx, http.MethodDelete, path, nil, "")
	return err
}

// withLimit maximizes the page size of the first paginated request. The
// rel="next" links of later pages carry the parameter forward on their own.
func withLimit(path string) string {
	if strings.Contains(path, "limit=") {
		return path
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%slimit=%d", path, separator, maxPageSize)
}

// maxPaginatedPages bounds how many pages GetPaginated will fetch; a real PR
// never comes close, so hitting it indicates a broken or malicious endpoint.
const maxPaginatedPages = 1000

// GetPaginated collects every page of a listing into out, a pointer to a
// slice, following the Link: rel="next" header Forgejo sets on its listings.
func (c *Client) GetPaginated(ctx context.Context, path string, out any) error {
	target := reflect.ValueOf(out)
	if target.Kind() != reflect.Pointer || target.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("forgejo: paginated output must be pointer to slice")
	}
	sliceValue := target.Elem()
	// visited defends against a server/proxy returning rel="next" links that
	// cycle (self-loops as well as longer A→B→A cycles), which would loop
	// forever.
	visited := make(map[string]struct{})
	nextPath := path
	first := true
	for nextPath != "" {
		if _, seen := visited[nextPath]; seen {
			return fmt.Errorf("forgejo: pagination cycle for %s at %s", path, nextPath)
		}
		if len(visited) >= maxPaginatedPages {
			return fmt.Errorf("forgejo: pagination for %s exceeded %d pages", path, maxPaginatedPages)
		}
		visited[nextPath] = struct{}{}
		// Only the first request is augmented with the limit; the rel="next"
		// links of later pages already carry it. Cycle detection keys on the
		// link-provided path, unaugmented.
		requestPath := nextPath
		if first {
			requestPath = withLimit(nextPath)
			first = false
		}
		body, resp, err := c.do(ctx, requestPath)
		if err != nil {
			return err
		}
		page := reflect.New(sliceValue.Type())
		if err := json.Unmarshal(body, page.Interface()); err != nil {
			return err
		}
		sliceValue.Set(reflect.AppendSlice(sliceValue, page.Elem()))
		nextPath = c.nextLink(resp.Header.Get("Link"))
	}
	return nil
}

func (c *Client) do(ctx context.Context, path string) ([]byte, *http.Response, error) {
	return c.doRequest(ctx, http.MethodGet, path, nil, "")
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, *http.Response, error) {
	if c.baseURL == "" {
		return nil, nil, errors.New("forgejo: no API base URL configured")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, nil, err
	}
	if c.token != "" {
		// "token" is the scheme Forgejo documents for API tokens; Bearer is
		// accepted too, but only for OAuth tokens on older releases.
		req.Header.Set("Authorization", "token "+c.token)
	}
	req.Header.Set("Accept", "application/json, text/plain")
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

// APIError is returned when the API responds with a >= 300 status. It carries
// the HTTP status so callers (e.g. the review publisher) can branch on it via
// errors.As.
type APIError struct {
	Method string
	URL    string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	message := fmt.Sprintf("forgejo: %s %s: status %d", e.Method, e.URL, e.Status)
	if text := strings.TrimSpace(e.Body); text != "" {
		message += ": " + text
	}
	if e.Status == http.StatusNotFound {
		message += " (check --repo, --id, the base URL, and token repo access)"
	}
	return message
}

func newAPIError(method, requestURL string, status int, body []byte) *APIError {
	return &APIError{Method: method, URL: requestURL, Status: status, Body: string(body)}
}

// IsNotFound reports whether err is a 404 from the API.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

// IsAPIError reports whether err is an answer from the API (as opposed to a
// transport failure), so a publisher can tell a rejected payload from a
// server it could not reach.
func IsAPIError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr)
}

func escapeRepo(repo string) string {
	parts := strings.Split(repo, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

// escapePath escapes a repository-relative path for a URL path segment list,
// keeping the separators intact.
func escapePath(path string) string {
	parts := strings.Split(path, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

// nextLink reads the rel="next" target of a Link header as a path relative to
// the client's API base, the form doRequest expects. Forgejo writes absolute
// links, its AppURL followed by the full request URI, so the link repeats the
// base's path ("/api/v1", or "/forgejo/api/v1" on an instance served under a
// subpath) and that prefix is stripped. Only the path and query are kept: the
// token goes to the configured host whatever host the link names.
func (c *Client) nextLink(header string) string {
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
		return c.apiRelative(parsed.RequestURI())
	}
	return ""
}

// apiRelative strips the API base's path from a request URI. When a proxy
// serves the API under another prefix than the one configured, the API
// version segment still marks where the endpoint path begins.
func (c *Client) apiRelative(requestURI string) string {
	basePath := ""
	if base, err := url.Parse(c.baseURL); err == nil {
		basePath = strings.TrimRight(base.Path, "/")
	}
	if basePath != "" {
		if rest, ok := strings.CutPrefix(requestURI, basePath); ok && (rest == "" || rest[0] == '/' || rest[0] == '?') {
			return rest
		}
	}
	path, _, _ := strings.Cut(requestURI, "?")
	if loc := apiVersionPathRegex.FindStringIndex(path); loc != nil {
		end := loc[1]
		if path[end-1] == '/' {
			end--
		}
		return requestURI[end:]
	}
	return requestURI
}
