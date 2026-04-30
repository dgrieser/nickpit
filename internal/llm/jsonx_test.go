package llm

import (
	"reflect"
	"testing"
)

func TestStripCodeFences(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no fences", `{"a":1}`, `{"a":1}`},
		{"json fence", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"plain fence", "```\n{\"a\":1}\n```", `{"a":1}`},
		{"javascript fence", "```javascript\n{\"a\":1}\n```", `{"a":1}`},
		{"trailing whitespace", "```json\n{\"a\":1}\n```\n  \n", `{"a":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripCodeFences(tt.in); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractJSONObject(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"prose before", "Sure! Here it is: {\"a\":1} hope it helps", `{"a":1}`, true},
		{"prose after", `{"a":1}\n\nLet me know.`, `{"a":1}`, true},
		{"nested braces", `prefix {"a":{"b":2}} suffix`, `{"a":{"b":2}}`, true},
		{"array", `Result:\n[1,2,3]\nDone`, `[1,2,3]`, true},
		{"braces in string", `{"a":"}{"} `, `{"a":"}{"}`, true},
		{"escaped quote", `{"a":"\"}"}`, `{"a":"\"}"}`, true},
		{"none", "no json here", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExtractJSONObject(tt.in)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v (got=%q)", ok, tt.ok, got)
			}
			if ok && got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRepairJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"trailing comma object", `{"a":1,}`, `{"a":1}`},
		{"trailing comma array", `[1,2,3,]`, `[1,2,3]`},
		{"single quotes", `{'a':'b'}`, `{"a":"b"}`},
		{"line comment", "{\"a\":1 // note\n,\"b\":2}", "{\"a\":1 \n,\"b\":2}"},
		{"block comment", "{\"a\": /* skip */ 1}", `{"a":  1}`},
		{"python literals", `{"a":True,"b":False,"c":None}`, `{"a":true,"b":false,"c":null}`},
		{"preserves string contents", `{"a":"True, 'x', // comment, /* */"}`, `{"a":"True, 'x', // comment, /* */"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(RepairJSON([]byte(tt.in)))
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLenientUnmarshal(t *testing.T) {
	type payload struct {
		A int            `json:"a"`
		B string         `json:"b"`
		C map[string]any `json:"c"`
	}
	tests := []struct {
		name string
		in   string
		want payload
	}{
		{
			"strict json",
			`{"a":1,"b":"hi","c":{"k":"v"}}`,
			payload{A: 1, B: "hi", C: map[string]any{"k": "v"}},
		},
		{
			"prose around",
			"Sure thing! Here is your JSON:\n\n{\"a\":1,\"b\":\"hi\",\"c\":{\"k\":\"v\"}}\n\nLet me know if you need anything else.",
			payload{A: 1, B: "hi", C: map[string]any{"k": "v"}},
		},
		{
			"json fenced",
			"```json\n{\"a\":1,\"b\":\"hi\",\"c\":{\"k\":\"v\"}}\n```",
			payload{A: 1, B: "hi", C: map[string]any{"k": "v"}},
		},
		{
			"trailing comma + single quotes",
			`{'a':1,'b':'hi','c':{'k':'v'},}`,
			payload{A: 1, B: "hi", C: map[string]any{"k": "v"}},
		},
		{
			"python literals",
			`{"a":1,"b":"hi","c":{"flag":True}}`,
			payload{A: 1, B: "hi", C: map[string]any{"flag": true}},
		},
		{
			"line comments",
			"{\n  \"a\": 1, // primary key\n  \"b\": \"hi\",\n  \"c\": {\"k\":\"v\"}\n}",
			payload{A: 1, B: "hi", C: map[string]any{"k": "v"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got payload
			if err := LenientUnmarshal(tt.in, &got); err != nil {
				t.Fatalf("LenientUnmarshal error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLenientUnmarshalEmpty(t *testing.T) {
	var v any
	if err := LenientUnmarshal("   ", &v); err == nil {
		t.Fatalf("expected error for empty content")
	}
}

func TestLenientUnmarshalReturnsErrorOnGarbage(t *testing.T) {
	var v any
	if err := LenientUnmarshal("absolutely no json at all", &v); err == nil {
		t.Fatalf("expected error for non-JSON content")
	}
}
