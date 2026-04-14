package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = "https://gitlab.com/api/v4"
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
		return nil, nil, fmt.Errorf("gitlab: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return body, resp, nil
}

func escapeProject(project string) string {
	return url.PathEscape(project)
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
