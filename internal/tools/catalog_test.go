package tools

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestReviewUpdateCatalogSelection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		names []string
		want  []string
	}{
		{"default", nil, []string{"inspect_file", "list_files", "search", "find_callers", "find_callees", "find_references", "git_log", "git_show"}},
		{"explicit", []string{RequestReviewUpdate}, []string{RequestReviewUpdate}},
		{"mixed and duplicate", []string{RequestReviewUpdate, "search", RequestReviewUpdate}, []string{"search", RequestReviewUpdate}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			definitions, err := Definitions(tc.names...)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, definition := range definitions {
				names = append(names, definition.Name)
			}
			if !reflect.DeepEqual(names, tc.want) {
				t.Fatalf("names = %v, want %v", names, tc.want)
			}
			listing, err := InstructionsListing(tc.names...)
			if err != nil {
				t.Fatal(err)
			}
			wantUpdate := tc.names != nil
			if strings.Contains(listing, RequestReviewUpdate) != wantUpdate {
				t.Fatalf("unexpected listing: %s", listing)
			}
			if strings.Count(listing, "- `") != len(tc.want) {
				t.Fatalf("unexpected listing count: %s", listing)
			}
		})
	}
}

func TestReviewUpdateCatalogSchemaAndGuidance(t *testing.T) {
	definitions, err := Definitions(RequestReviewUpdate)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Type                 string   `json:"type"`
		AdditionalProperties bool     `json:"additionalProperties"`
		Required             []string `json:"required"`
		Properties           map[string]struct {
			Type  string `json:"type"`
			Items struct {
				Type string `json:"type"`
			} `json:"items"`
			MinItems *int `json:"minItems"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(definitions[0].Parameters, &schema); err != nil {
		t.Fatal(err)
	}
	if schema.Type != "object" || schema.AdditionalProperties || !reflect.DeepEqual(schema.Required, []string{"finding_ids", "reason"}) || len(schema.Properties) != 2 {
		t.Fatalf("invalid schema: %s", definitions[0].Parameters)
	}
	ids := schema.Properties["finding_ids"]
	if ids.Type != "array" || ids.Items.Type != "string" || ids.MinItems != nil || schema.Properties["reason"].Type != "string" {
		t.Fatalf("invalid parameter types: %s", definitions[0].Parameters)
	}
	if got := ArgumentSchema(RequestReviewUpdate); got != `{"finding_ids": ["<finding ID>"], "reason": "<evidence in English>"}` {
		t.Fatalf("argument hint = %s", got)
	}
	listing, err := InstructionsListing(RequestReviewUpdate)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"evidence warrants a correction", "empty finding list", "ALWAYS write the reason in English", "scheduled, queue_failed, or error", "scheduled, not completed", "Otherwise, do not claim", "Resolved findings cannot be reopened"} {
		if !strings.Contains(listing, text) {
			t.Errorf("listing missing %q", text)
		}
	}
	for _, text := range []string{"written in English", "empty list", "scheduled, queue_failed, or error", "scheduled, not completed", "Resolved findings cannot be reopened"} {
		if !strings.Contains(definitions[0].Description, text) {
			t.Errorf("description missing %q", text)
		}
	}
}
