package llm

import (
	"encoding/json"
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
		{"brace in prose before json", `Here is the configuration { as requested: {"key": "value"}`, `{"key": "value"}`, true},
		{"unbalanced first then balanced", `prefix { unbalanced [1,2,3]`, `[1,2,3]`, true},
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
		{"escapes embedded double quotes in single-quoted string", `{'text': 'He said "hello"'}`, `{"text": "He said \"hello\""}`},
		{"keeps already escaped double quotes in single-quoted string", `{'text': 'He said \"hi\"'}`, `{"text": "He said \"hi\""}`},
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
		{
			"single quotes around embedded double quotes",
			`{'a': 1, 'b': 'He said "hi"', 'c': {'k': 'v'}}`,
			payload{A: 1, B: `He said "hi"`, C: map[string]any{"k": "v"}},
		},
		{
			"prose with stray opening brace before valid json",
			`Sure! Here is the configuration { as requested: {"a":1,"b":"hi","c":{"k":"v"}}`,
			payload{A: 1, B: "hi", C: map[string]any{"k": "v"}},
		},
		{
			"markdown heading and bracketed prose before json",
			"# Review\n\n[P1] Finding summary\n\n```json\n{\"a\":1,\"b\":\"hi\",\"c\":{\"k\":\"v\"}}\n```\n\nDone.",
			payload{A: 1, B: "hi", C: map[string]any{"k": "v"}},
		},
		{
			"fenced markdown before json",
			"```markdown\n[P1] Finding summary\n\n{\"a\":1,\"b\":\"hi\",\"c\":{\"k\":\"v\"}}\n```",
			payload{A: 1, B: "hi", C: map[string]any{"k": "v"}},
		},
		{
			"priority label before json",
			"[P1] Finding summary\n\n{\"a\":1,\"b\":\"hi\",\"c\":{\"k\":\"v\"}}",
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

type mergePayload struct {
	Items   []string          `json:"items"`
	Name    string            `json:"name"`
	Count   int               `json:"count"`
	Nested  *mergeNested      `json:"nested,omitempty"`
	Tags    map[string]string `json:"tags,omitempty"`
	RawData json.RawMessage   `json:"raw,omitempty"`
}

type mergeNested struct {
	Detail string `json:"detail"`
	Level  int    `json:"level"`
}

func TestLenientUnmarshalMergeSingleBlock(t *testing.T) {
	var got mergePayload
	if err := LenientUnmarshalMerge(`{"items":["a","b"],"name":"x","count":3}`, &got); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := mergePayload{Items: []string{"a", "b"}, Name: "x", Count: 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestLenientUnmarshalMergeConcatSlices(t *testing.T) {
	content := "prose\n```json\n" + `{"items":["a","b"],"name":"first"}` + "\n```\n" +
		"more prose\n```json\n" + `{"items":["c"],"count":7}` + "\n```"
	var got mergePayload
	if err := LenientUnmarshalMerge(content, &got); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := mergePayload{Items: []string{"a", "b", "c"}, Name: "first", Count: 7}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestLenientUnmarshalMergeScalarLastNonZeroWins(t *testing.T) {
	content := `{"name":"first","count":3}` + "\n\n" + `{"count":9}`
	var got mergePayload
	if err := LenientUnmarshalMerge(content, &got); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Name != "first" {
		t.Fatalf("Name = %q want %q (later zero must not blank earlier)", got.Name, "first")
	}
	if got.Count != 9 {
		t.Fatalf("Count = %d want 9 (later non-zero wins)", got.Count)
	}
}

func TestLenientUnmarshalMergeMapUnion(t *testing.T) {
	content := `{"tags":{"a":"1","b":"2"}}` + "\n" + `{"tags":{"b":"99","c":"3"}}`
	var got mergePayload
	if err := LenientUnmarshalMerge(content, &got); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := map[string]string{"a": "1", "b": "99", "c": "3"}
	if !reflect.DeepEqual(got.Tags, want) {
		t.Fatalf("Tags = %+v want %+v", got.Tags, want)
	}
}

func TestLenientUnmarshalMergeNestedStruct(t *testing.T) {
	content := `{"name":"x"}` + "\n" + `{"nested":{"detail":"d","level":2}}`
	var got mergePayload
	if err := LenientUnmarshalMerge(content, &got); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Nested == nil || got.Nested.Detail != "d" || got.Nested.Level != 2 {
		t.Fatalf("Nested = %+v", got.Nested)
	}
}

func TestLenientUnmarshalMergeRawMessageScalar(t *testing.T) {
	content := `{"raw":{"k":1}}` + "\n" + `{"raw":{"k":2}}`
	var got mergePayload
	if err := LenientUnmarshalMerge(content, &got); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if string(got.RawData) != `{"k":2}` {
		t.Fatalf("RawData = %s want last-wins scalar", string(got.RawData))
	}
}

func TestLenientUnmarshalMergeAllFail(t *testing.T) {
	var got mergePayload
	if err := LenientUnmarshalMerge("absolutely no json at all", &got); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLenientUnmarshalMergeRejectsTypedNilPointer(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("LenientUnmarshalMerge panicked: %v", r)
		}
	}()
	var got *mergePayload
	err := LenientUnmarshalMerge(`{"name":"first"}`+"\n"+`{"count":7}`, got)
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "v must be a non-nil pointer" {
		t.Fatalf("error = %q, want non-nil pointer error", err.Error())
	}
}

func TestLenientUnmarshalMergeFallbackTypeAppendsSlice(t *testing.T) {
	type container struct {
		Items []mergeNested `json:"items"`
	}
	fb := FallbackType{
		NewInstance: func() any { return new(mergeNested) },
		Attach: func(into, parsed any) bool {
			n := parsed.(*mergeNested)
			if n.Detail == "" {
				return false
			}
			c := into.(*container)
			c.Items = append(c.Items, *n)
			return true
		},
	}
	content := `{"items":[{"detail":"a","level":1}]}` + "\n" + `{"detail":"b","level":2}`
	var got container
	if err := LenientUnmarshalMerge(content, &got, fb); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := container{Items: []mergeNested{{Detail: "a", Level: 1}, {Detail: "b", Level: 2}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

type mergeableRecorder struct {
	Sum         int               `json:"sum"`
	Tag         string            `json:"tag"`
	Calls       int               `json:"-"`
	SeenKeysAll []map[string]bool `json:"-"`
}

func (m *mergeableRecorder) MergeFrom(other any, presentKeys map[string]bool) (bool, error) {
	src := other.(*mergeableRecorder)
	m.Calls++
	keysCopy := make(map[string]bool, len(presentKeys))
	for k, v := range presentKeys {
		keysCopy[k] = v
	}
	m.SeenKeysAll = append(m.SeenKeysAll, keysCopy)
	claimed := false
	if presentKeys["sum"] {
		m.Sum += src.Sum
		claimed = true
	}
	if presentKeys["tag"] {
		m.Tag = src.Tag
		claimed = true
	}
	return claimed, nil
}

func TestLenientUnmarshalMergePrefersMergeableOverReflect(t *testing.T) {
	var got mergeableRecorder
	content := `{"sum":3,"tag":"a"}` + "\n" + `{"sum":4,"tag":"b"}`
	if err := LenientUnmarshalMerge(content, &got); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Calls != 2 {
		t.Fatalf("Calls = %d, want 2", got.Calls)
	}
	if got.Sum != 7 {
		t.Fatalf("Sum = %d, want 7 (Mergeable sums; reflect would last-non-zero-win to 4)", got.Sum)
	}
	if got.Tag != "b" {
		t.Fatalf("Tag = %q, want b", got.Tag)
	}
}

func TestLenientUnmarshalMergeMergeableReceivesPresentKeys(t *testing.T) {
	var got mergeableRecorder
	content := `{"sum":3}` + "\n" + `{"tag":"x"}`
	if err := LenientUnmarshalMerge(content, &got); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(got.SeenKeysAll) != 2 {
		t.Fatalf("SeenKeysAll = %v", got.SeenKeysAll)
	}
	if !got.SeenKeysAll[0]["sum"] || got.SeenKeysAll[0]["tag"] {
		t.Fatalf("first call keys = %v", got.SeenKeysAll[0])
	}
	if got.SeenKeysAll[1]["sum"] || !got.SeenKeysAll[1]["tag"] {
		t.Fatalf("second call keys = %v", got.SeenKeysAll[1])
	}
}

func TestLenientUnmarshalMergeMergeableReceivesPresentKeysFromRepairedCandidate(t *testing.T) {
	var got mergeableRecorder
	content := `{"sum":3,}` + "\n" + `{"tag":"x"}`
	if err := LenientUnmarshalMerge(content, &got); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Sum != 3 {
		t.Fatalf("Sum = %d, want repaired candidate to contribute", got.Sum)
	}
	if len(got.SeenKeysAll) != 2 {
		t.Fatalf("SeenKeysAll = %v", got.SeenKeysAll)
	}
	if !got.SeenKeysAll[0]["sum"] {
		t.Fatalf("first call keys = %v, want sum from repaired JSON", got.SeenKeysAll[0])
	}
}

func TestLenientUnmarshalMergeMergeableNotClaimedFallsThroughToFallbacks(t *testing.T) {
	fb := FallbackType{
		NewInstance: func() any { return new(mergeableRecorder) },
		Attach: func(into, parsed any) bool {
			rr := into.(*mergeableRecorder)
			rr.Tag = "from-fallback"
			return true
		},
	}
	var got mergeableRecorder
	// Second candidate has no keys that mergeableRecorder claims; MergeFrom
	// returns (false, nil) → fallback runs.
	content := `{"sum":3,"tag":"first"}` + "\n" + `{"unrelated":99}`
	if err := LenientUnmarshalMerge(content, &got, fb); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Tag != "from-fallback" {
		t.Fatalf("Tag = %q, want fallback to claim", got.Tag)
	}
}

func TestLenientUnmarshalMergeFallbackReturnsFalseFallsThrough(t *testing.T) {
	type container struct {
		Items []mergeNested `json:"items"`
		Tags  []string      `json:"tags"`
	}
	rejecting := FallbackType{
		NewInstance: func() any { return new(mergeNested) },
		Attach:      func(into, parsed any) bool { return false },
	}
	accepting := FallbackType{
		NewInstance: func() any { return new(mergeNested) },
		Attach: func(into, parsed any) bool {
			c := into.(*container)
			c.Items = append(c.Items, *parsed.(*mergeNested))
			return true
		},
	}
	content := `{"items":[]}` + "\n" + `{"detail":"a","level":1}`
	var got container
	if err := LenientUnmarshalMerge(content, &got, rejecting, accepting); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Detail != "a" {
		t.Fatalf("expected second fallback to claim, got %+v", got)
	}
}
