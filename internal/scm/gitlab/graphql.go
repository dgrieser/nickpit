package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// graphQLURL derives the instance's GraphQL endpoint from the REST base URL.
// GitLab serves GraphQL at /api/graphql, a sibling of the versioned REST root
// rather than a path below it.
func graphQLURL(baseURL string) string {
	if match := apiVersionPathRegex.FindStringIndex(baseURL); match != nil {
		return baseURL[:match[0]] + "/api/graphql"
	}
	return strings.TrimRight(baseURL, "/") + "/api/graphql"
}

// GraphQLError reports a GraphQL response carrying an "errors" array. GitLab
// answers such failures with HTTP 200, so an unknown field, an unlicensed
// feature, or a permission denial never surfaces as an APIError.
type GraphQLError struct {
	Messages []string
}

func (e *GraphQLError) Error() string {
	return "gitlab: graphql: " + strings.Join(e.Messages, "; ")
}

// GraphQL posts query with its variables to the instance's GraphQL endpoint and
// decodes the response's "data" object into out (nil discards it).
func (c *Client) GraphQL(ctx context.Context, query string, variables map[string]any, out any) error {
	payload := map[string]any{"query": query}
	if len(variables) > 0 {
		payload["variables"] = variables
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("gitlab: encoding graphql request: %w", err)
	}
	respBody, _, err := c.doRequestURL(ctx, http.MethodPost, graphQLURL(c.baseURL), bytes.NewReader(body), "application/json")
	if err != nil {
		return err
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("gitlab: decoding graphql response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		messages := make([]string, 0, len(envelope.Errors))
		for _, item := range envelope.Errors {
			messages = append(messages, item.Message)
		}
		return &GraphQLError{Messages: messages}
	}
	if out == nil {
		return nil
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("gitlab: graphql response carried no data")
	}
	return json.Unmarshal(envelope.Data, out)
}
