package llm

import (
	"encoding/json"
	"errors"
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

	extracted, ok := ExtractJSONObject(stripped)
	if ok {
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
// honoring the standard JSON escape sequences. Returns false if no balanced
// object or array is found.
func ExtractJSONObject(content string) (string, bool) {
	start := -1
	var open, close byte
	for i := 0; i < len(content); i++ {
		c := content[i]
		if c == '{' || c == '[' {
			start = i
			open = c
			if c == '{' {
				close = '}'
			} else {
				close = ']'
			}
			break
		}
	}
	if start < 0 {
		return "", false
	}

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
				return content[start : i+1], true
			}
		}
	}
	return "", false
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
