package llm

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"unicode"
)

// LenientUnmarshal tries to decode arbitrary model-generated text into v.
// It first attempts a strict json.Unmarshal, then progressively strips
// markdown fences, leading/trailing prose, and applies common repairs
// (trailing commas, single-quoted strings, line/block comments,
// Python-style True/False/None) before retrying.
//
// On total failure, the original strict-parse error is returned so callers
// can surface the most informative message to the model.
func LenientUnmarshal(content string, v any) error {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return errors.New("empty content")
	}
	if err := json.Unmarshal([]byte(trimmed), v); err == nil {
		return nil
	}

	stripped := StripCodeFences(trimmed)
	if stripped != trimmed {
		if err := json.Unmarshal([]byte(stripped), v); err == nil {
			return nil
		}
	}

	for _, extracted := range extractJSONCandidates(stripped) {
		if err := json.Unmarshal([]byte(extracted), v); err == nil {
			return nil
		}
		repaired := RepairJSON([]byte(extracted))
		if err := json.Unmarshal(repaired, v); err == nil {
			return nil
		}
	}

	repaired := RepairJSON([]byte(stripped))
	if err := json.Unmarshal(repaired, v); err == nil {
		return nil
	}

	return json.Unmarshal([]byte(trimmed), v)
}

// Mergeable lets a target type define its own merge semantics for
// LenientUnmarshalMerge. When the target's pointer implements this
// interface, the merger calls MergeFrom for each successfully parsed
// candidate instead of using the generic reflection-based merge.
//
// other is a *T pointing at the freshly-decoded candidate (same concrete
// type as the accumulator). presentKeys is the set of top-level JSON keys
// the candidate actually emitted — types should use it to distinguish
// "field omitted" from "field set to its zero value". When the candidate
// is a top-level JSON array (not an object), presentKeys is empty.
//
// Return value: claimed=true when the candidate contributed at least one
// field to the accumulator. claimed=false signals "no relevant keys" so
// LenientUnmarshalMerge falls through to its FallbackType list. err is
// returned to the caller of LenientUnmarshalMerge unchanged.
type Mergeable interface {
	MergeFrom(other any, presentKeys map[string]bool) (claimed bool, err error)
}

// FallbackType lets a caller of LenientUnmarshalMerge declare additional
// shapes to try when a JSON candidate parses successfully into *v but yields
// the zero value (i.e. no fields overlapped), or fails to parse into *v at
// all. NewInstance returns a fresh non-nil pointer to a value of the
// fallback type. Attach receives the accumulator (the same pointer the
// caller passed as v) and the parsed fallback instance; it returns true if
// the parsed value carried meaningful content and was attached.
type FallbackType struct {
	NewInstance func() any
	Attach      func(into any, parsed any) bool
}

// LenientUnmarshalMerge behaves like LenientUnmarshal when the input
// contains a single JSON candidate. When multiple candidates are found,
// each candidate that parses into a fresh *v-typed value and is not the
// zero value is merged into an accumulator: slice fields concatenate,
// scalar fields use last-non-zero-wins, pointer/interface fields use
// last-non-nil-wins, maps union with last-wins on key collisions, and
// nested structs recurse. Candidates that parse to a zero *v (no field
// overlap) or fail to parse into *v are tried against each fallback in
// order; the first whose Attach returns true claims the candidate.
//
// Custom UnmarshalJSON implementations on element types run per candidate
// during unmarshal, so the merge layer always sees post-unmarshal values.
//
// If no candidate parses (primary or fallback) and the repaired full
// content also fails, the strict-parse error from the trimmed input is
// returned.
func LenientUnmarshalMerge(content string, v any, fallbacks ...FallbackType) error {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return errors.New("empty content")
	}
	if err := json.Unmarshal([]byte(trimmed), v); err == nil {
		return nil
	}
	// Run candidate extraction directly on the trimmed input. StripCodeFences
	// is unsafe here: if content has prose or another block after the closing
	// fence, the strip would discard it. Fence characters do not affect
	// balanced-brace scanning, so candidates inside fences are still found.
	candidates := extractJSONCandidates(trimmed)
	if len(candidates) <= 1 {
		return LenientUnmarshal(content, v)
	}

	rt := reflect.TypeOf(v)
	if rt == nil || rt.Kind() != reflect.Pointer {
		return errors.New("v must be a non-nil pointer")
	}
	elemType := rt.Elem()
	accumulator := reflect.New(elemType)
	accumulator.Elem().Set(reflect.ValueOf(v).Elem())
	merged := false

	_, isMergeable := accumulator.Interface().(Mergeable)
	for _, extracted := range candidates {
		parsed, decoded, ok := tryParseCandidate(extracted, elemType)
		if ok {
			if isMergeable {
				keys := topLevelJSONKeys(decoded)
				claimed, err := accumulator.Interface().(Mergeable).MergeFrom(parsed.Interface(), keys)
				if err != nil {
					return err
				}
				if claimed {
					merged = true
					continue
				}
			} else if !parsed.Elem().IsZero() {
				mergeJSONValue(accumulator.Elem(), parsed.Elem())
				merged = true
				continue
			}
		}
		for _, fb := range fallbacks {
			fbParsed, ok := tryParseFallback(extracted, fb.NewInstance)
			if !ok {
				continue
			}
			if fb.Attach(accumulator.Interface(), fbParsed) {
				merged = true
				break
			}
		}
	}

	if merged {
		reflect.ValueOf(v).Elem().Set(accumulator.Elem())
		return nil
	}
	return LenientUnmarshal(content, v)
}

// topLevelJSONKeys returns the set of top-level keys present in a JSON
// object candidate. For arrays or non-object candidates the returned map
// is empty (and non-nil so callers can index it without nil checks).
func topLevelJSONKeys(decoded []byte) map[string]bool {
	keys := make(map[string]bool)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(decoded, &raw); err != nil {
		return keys
	}
	for k := range raw {
		keys[k] = true
	}
	return keys
}

func tryParseCandidate(extracted string, elemType reflect.Type) (reflect.Value, []byte, bool) {
	parsed, decoded, ok := decodeJSONCandidate(extracted, func() any {
		return reflect.New(elemType).Interface()
	})
	if ok {
		return reflect.ValueOf(parsed), decoded, true
	}
	return reflect.Value{}, nil, false
}

func tryParseFallback(extracted string, alloc func() any) (any, bool) {
	parsed, _, ok := decodeJSONCandidate(extracted, alloc)
	if ok {
		return parsed, true
	}
	return nil, false
}

func decodeJSONCandidate(extracted string, alloc func() any) (any, []byte, bool) {
	decoded := []byte(extracted)
	parsed := alloc()
	if err := json.Unmarshal(decoded, parsed); err == nil {
		return parsed, decoded, true
	}
	decoded = RepairJSON(decoded)
	parsed = alloc()
	if err := json.Unmarshal(decoded, parsed); err == nil {
		return parsed, decoded, true
	}
	return nil, nil, false
}

var rawMessageType = reflect.TypeOf(json.RawMessage(nil))

func mergeJSONValue(dst, src reflect.Value) {
	if !dst.IsValid() || !src.IsValid() {
		return
	}
	if dst.Type() == rawMessageType {
		if src.Len() > 0 {
			dst.Set(src)
		}
		return
	}
	switch dst.Kind() {
	case reflect.Struct:
		for i := 0; i < dst.NumField(); i++ {
			if !dst.Field(i).CanSet() {
				continue
			}
			mergeJSONValue(dst.Field(i), src.Field(i))
		}
	case reflect.Pointer:
		if src.IsNil() {
			return
		}
		if dst.IsNil() {
			dst.Set(src)
			return
		}
		mergeJSONValue(dst.Elem(), src.Elem())
	case reflect.Slice:
		if src.IsNil() || src.Len() == 0 {
			return
		}
		if dst.IsNil() {
			dst.Set(src)
			return
		}
		dst.Set(reflect.AppendSlice(dst, src))
	case reflect.Map:
		if src.IsNil() || src.Len() == 0 {
			return
		}
		if dst.IsNil() {
			dst.Set(reflect.MakeMap(dst.Type()))
		}
		for _, key := range src.MapKeys() {
			dst.SetMapIndex(key, src.MapIndex(key))
		}
	case reflect.Interface:
		if src.IsNil() {
			return
		}
		dst.Set(src)
	default:
		if src.IsZero() {
			return
		}
		dst.Set(src)
	}
}

// StripCodeFences removes leading and trailing markdown code fences such as
// ```json, ```javascript, or plain ``` from content. The inner payload is
// returned trimmed of surrounding whitespace.
func StripCodeFences(content string) string {
	s := strings.TrimSpace(content)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	rest := strings.TrimPrefix(s, "```")
	if newline := strings.IndexByte(rest, '\n'); newline >= 0 {
		lang := strings.TrimSpace(rest[:newline])
		if lang == "" || isLanguageTag(lang) {
			rest = rest[newline+1:]
		}
	}
	if idx := strings.LastIndex(rest, "```"); idx >= 0 {
		rest = rest[:idx]
	}
	return strings.TrimSpace(rest)
}

func isLanguageTag(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && r != '+' {
			return false
		}
	}
	return true
}

// ExtractJSONObject locates the first balanced JSON object or array in content
// and returns it as a substring. Braces inside string literals are ignored,
// honoring the standard JSON escape sequences. If the first opening bracket
// does not yield a balanced structure (e.g. it appears in surrounding prose),
// subsequent candidates are tried. Returns false if no balanced object or
// array is found.
func ExtractJSONObject(content string) (string, bool) {
	for pos := 0; pos < len(content); pos++ {
		c := content[pos]
		if c != '{' && c != '[' {
			continue
		}
		var close byte
		if c == '{' {
			close = '}'
		} else {
			close = ']'
		}
		if extracted, _, ok := scanBalanced(content, pos, c, close); ok {
			return extracted, true
		}
	}
	return "", false
}

func extractJSONCandidates(content string) []string {
	var candidates []string
	for pos := 0; pos < len(content); pos++ {
		c := content[pos]
		if c != '{' && c != '[' {
			continue
		}
		var close byte
		if c == '{' {
			close = '}'
		} else {
			close = ']'
		}
		if extracted, end, ok := scanBalanced(content, pos, c, close); ok {
			candidates = append(candidates, extracted)
			pos = end
		}
	}
	return candidates
}

func scanBalanced(content string, start int, open, close byte) (string, int, bool) {
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(content); i++ {
		c := content[i]
		if escape {
			escape = false
			continue
		}
		if inString {
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return content[start : i+1], i, true
			}
		}
	}
	return "", 0, false
}

// RepairJSON applies best-effort fixes to common malformations produced by
// language models: trailing commas before closing brackets, single-quoted
// string literals, // and /* */ comments, and Python-style True/False/None
// literals. The function preserves the contents of valid JSON strings.
func RepairJSON(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	stringQuote := byte(0)
	escape := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if escape {
			out = append(out, c)
			escape = false
			continue
		}
		if inString {
			if c == '\\' {
				out = append(out, c)
				escape = true
				continue
			}
			if c == stringQuote {
				inString = false
				if stringQuote == '\'' {
					out = append(out, '"')
					continue
				}
				out = append(out, c)
				continue
			}
			// Inside a single-quoted string, unescaped double quotes must be
			// escaped to keep the rewritten JSON valid.
			if stringQuote == '\'' && c == '"' {
				out = append(out, '\\', '"')
				continue
			}
			out = append(out, c)
			continue
		}

		// Strip line comments: // ... \n
		if c == '/' && i+1 < len(data) && data[i+1] == '/' {
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				out = append(out, '\n')
			}
			continue
		}
		// Strip block comments: /* ... */
		if c == '/' && i+1 < len(data) && data[i+1] == '*' {
			i += 2
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				i++
			}
			i++ // points at '/', loop's i++ moves past it
			continue
		}

		if c == '"' {
			inString = true
			stringQuote = '"'
			out = append(out, c)
			continue
		}
		if c == '\'' {
			inString = true
			stringQuote = '\''
			out = append(out, '"')
			continue
		}

		// Trailing comma: "," followed by whitespace, then } or ]
		if c == ',' {
			j := i + 1
			for j < len(data) && isJSONSpace(data[j]) {
				j++
			}
			if j < len(data) && (data[j] == '}' || data[j] == ']') {
				continue
			}
		}

		// Replace Python-style literals when they appear as standalone tokens.
		if isIdentStart(c) {
			j := i
			for j < len(data) && isIdentPart(data[j]) {
				j++
			}
			word := string(data[i:j])
			switch word {
			case "True":
				out = append(out, []byte("true")...)
				i = j - 1
				continue
			case "False":
				out = append(out, []byte("false")...)
				i = j - 1
				continue
			case "None":
				out = append(out, []byte("null")...)
				i = j - 1
				continue
			}
		}

		out = append(out, c)
	}
	return out
}

func isJSONSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isIdentStart(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}
