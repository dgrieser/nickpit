package github

import (
	"context"
	"encoding/json"
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

func (c *Client) GetPaginated(ctx context.Context, path string, out any) error {
	target := reflect.ValueOf(out)
	if target.Kind() != reflect.Pointer || target.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("github: paginated output must be pointer to slice")
	}
	sliceValue := target.Elem()
	nextPath := path
	for nextPath != "" {
		current := nextPath
		body, resp, err := c.do(ctx, current)
		if err != nil {
			return err
		}
		page := reflect.New(sliceValue.Type())
		if err := json.Unmarshal(body, page.Interface()); err != nil {
			return err
		}
		sliceValue.Set(reflect.AppendSlice(sliceValue, page.Elem()))
		next := nextLink(resp.Header.Get("Link"))
		if next == current {
			// Defend against a server/proxy returning a rel="next" link that
			// points back to the page just fetched, which would loop forever.
			break
		}
		nextPath = next
	}
	return nil
}

func (c *Client) do(ctx context.Context, path string) ([]byte, *http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("github: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, nil, err
	}
	return body, resp, nil
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
