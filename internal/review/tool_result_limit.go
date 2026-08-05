package review

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxReferenceFunctions = 25

const toolResultTruncatedNote = "tool result exceeded configured item or size limits; narrow path, range, depth, or result count for more detail"

// limitToolResultJSON applies tool-specific item limits and the profile-wide
// encoded-size limit to the final JSON sent back to the model. Keeping this at
// the dispatch boundary covers fallbacks, rewritten calls, errors, and future
// tools without coupling retrieval APIs to an LLM context budget.
func limitToolResultJSON(raw string, maxKiB int) string {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return raw
	}
	root, ok := value.(map[string]any)
	if !ok {
		return raw
	}

	changed := applyToolItemLimits(root)
	if maxKiB > int(^uint(0)>>1)/1024 {
		maxKiB = 0
	}
	limit := int64(maxKiB) * 1024
	if !changed && (maxKiB <= 0 || int64(len(raw)) <= limit) {
		return raw
	}
	if changed {
		markToolResultTruncated(root)
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		return raw
	}
	if maxKiB <= 0 || int64(len(encoded)) <= limit {
		return string(encoded)
	}

	markToolResultTruncated(root)
	for attempts := 0; attempts < 10_000; attempts++ {
		encoded, err = json.Marshal(root)
		if err != nil {
			return raw
		}
		if int64(len(encoded)) <= limit {
			return string(encoded)
		}
		over := len(encoded) - int(limit)
		if !reduceToolResult(root, over, len(encoded)) {
			break
		}
	}

	fallback := map[string]any{
		"truncated":      true,
		"truncated_note": "tool result exceeded max_tool_result_kib and could not retain its original structure; rerun with narrower arguments",
	}
	encoded, _ = json.Marshal(fallback)
	if int64(len(encoded)) <= limit {
		return string(encoded)
	}
	return `{"truncated":true}`
}

func applyToolItemLimits(root map[string]any) bool {
	if !isReferencePayload(root) {
		return false
	}
	functions := arrayField(root, "functions")
	if len(functions) <= maxReferenceFunctions {
		return false
	}
	omitted := len(functions) - maxReferenceFunctions
	root["functions"] = functions[:maxReferenceFunctions]
	addIntField(root, "omitted_contexts", omitted)
	return true
}

func reduceToolResult(root map[string]any, over, size int) bool {
	switch {
	case isReferencePayload(root):
		if dropArrayTail(root, "functions", over, size, "omitted_contexts") {
			return true
		}
		if dropArrayTail(root, "outside_functions", over, size, "omitted_contexts") {
			return true
		}
		if target, ok := objectField(root, "target"); ok {
			if definition, ok := objectField(target, "definition"); ok {
				return trimStringField(definition, "content", over, true)
			}
		}
	case root["root"] != nil:
		if hierarchy, ok := root["root"].(map[string]any); ok {
			if trimDeepestHierarchyContent(hierarchy, over) {
				return true
			}
			if omitted := pruneDeepestHierarchyNode(hierarchy); omitted > 0 {
				addIntField(root, "omitted_nodes", omitted)
				return true
			}
		}
	case root["results"] != nil:
		if dropArrayTail(root, "results", over, size, "omitted_results") {
			root["result_count"] = len(arrayField(root, "results"))
			return true
		}
		if dropArrayTail(root, "truncated_files", over, size, "omitted_truncated_files") {
			return true
		}
	case root["files"] != nil:
		return dropArrayTail(root, "files", over, size, "omitted_files")
	case root["commits"] != nil:
		if reduceHistoryResult(root, over, size) {
			return true
		}
	case root["content"] != nil:
		if trimStringField(root, "content", over, true) {
			if start, ok := intField(root, "start_line"); ok {
				content, _ := root["content"].(string)
				root["end_line"] = start + strings.Count(content, "\n")
			}
			return true
		}
	}
	return reduceGenericJSON(root, over, size)
}

func reduceHistoryResult(root map[string]any, over, size int) bool {
	commits := arrayField(root, "commits")
	if len(commits) > 1 {
		if dropArrayTail(root, "commits", over, size, "omitted_commits") {
			root["commit_count"] = len(arrayField(root, "commits"))
			return true
		}
	}
	if trimLargestNamedString(root, over, map[string]bool{"content": true}) {
		return true
	}
	if len(commits) == 1 {
		if commit, ok := commits[0].(map[string]any); ok {
			for _, key := range []string{"diff_hunks", "diff_files", "files"} {
				if dropArrayTail(commit, key, over, size, "") {
					return true
				}
			}
			if trimStringField(commit, "body", over, true) {
				return true
			}
		}
	}
	return false
}

func markToolResultTruncated(root map[string]any) {
	root["truncated"] = true
	existing, _ := root["truncated_note"].(string)
	if existing == "" {
		root["truncated_note"] = toolResultTruncatedNote
		return
	}
	if !strings.Contains(existing, toolResultTruncatedNote) {
		root["truncated_note"] = existing + "; " + toolResultTruncatedNote
	}
}

func isReferencePayload(root map[string]any) bool {
	_, target := root["target"]
	_, functions := root["functions"]
	_, outside := root["outside_functions"]
	return target && functions && outside
}

func arrayField(object map[string]any, key string) []any {
	value, _ := object[key].([]any)
	return value
}

func objectField(object map[string]any, key string) (map[string]any, bool) {
	value, ok := object[key].(map[string]any)
	return value, ok
}

func intField(object map[string]any, key string) (int, bool) {
	switch value := object[key].(type) {
	case json.Number:
		parsed, err := value.Int64()
		return int(parsed), err == nil
	case float64:
		return int(value), true
	case int:
		return value, true
	default:
		return 0, false
	}
}

func addIntField(object map[string]any, key string, delta int) {
	current, _ := intField(object, key)
	object[key] = current + delta
}

func dropArrayTail(object map[string]any, key string, over, size int, omittedKey string) bool {
	values := arrayField(object, key)
	if len(values) == 0 {
		return false
	}
	drop := max(1, int(int64(len(values))*int64(max(1, over))/int64(max(1, size))))
	drop = min(drop, len(values))
	object[key] = values[:len(values)-drop]
	if omittedKey != "" {
		addIntField(object, omittedKey, drop)
	}
	return true
}

func trimStringField(object map[string]any, key string, over int, preferLine bool) bool {
	value, ok := object[key].(string)
	if !ok || value == "" {
		return false
	}
	remove := min(len(value), max(1, over+64))
	end := len(value) - remove
	for end > 0 && !utf8.ValidString(value[:end]) {
		end--
	}
	if preferLine && end > 0 {
		if newline := strings.LastIndexByte(value[:end], '\n'); newline >= 0 {
			end = newline
		}
	}
	object[key] = value[:end]
	return true
}

type hierarchyContent struct {
	location map[string]any
	depth    int
	length   int
}

func trimDeepestHierarchyContent(root map[string]any, over int) bool {
	var best hierarchyContent
	var walk func(map[string]any, int)
	walk = func(node map[string]any, depth int) {
		if location, ok := objectField(node, "code_location"); ok {
			if content, ok := location["content"].(string); ok && content != "" && (depth > best.depth || depth == best.depth && len(content) > best.length) {
				best = hierarchyContent{location: location, depth: depth, length: len(content)}
			}
		}
		for _, child := range arrayField(node, "children") {
			if childNode, ok := child.(map[string]any); ok {
				walk(childNode, depth+1)
			}
		}
	}
	walk(root, 0)
	return best.location != nil && trimStringField(best.location, "content", over, true)
}

func pruneDeepestHierarchyNode(root map[string]any) int {
	var bestParent map[string]any
	bestDepth := -1
	var walk func(map[string]any, int)
	walk = func(node map[string]any, depth int) {
		children := arrayField(node, "children")
		if len(children) > 0 && depth >= bestDepth {
			bestParent = node
			bestDepth = depth
		}
		for _, child := range children {
			if childNode, ok := child.(map[string]any); ok {
				walk(childNode, depth+1)
			}
		}
	}
	walk(root, 0)
	if bestParent == nil {
		return 0
	}
	children := arrayField(bestParent, "children")
	last := children[len(children)-1]
	bestParent["children"] = children[:len(children)-1]
	return countHierarchyNodes(last)
}

func countHierarchyNodes(value any) int {
	node, ok := value.(map[string]any)
	if !ok {
		return 0
	}
	count := 1
	for _, child := range arrayField(node, "children") {
		count += countHierarchyNodes(child)
	}
	return count
}

type jsonArrayCandidate struct {
	object map[string]any
	key    string
	length int
}

func reduceGenericJSON(root map[string]any, over, size int) bool {
	var best jsonArrayCandidate
	var visit func(any)
	visit = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for _, key := range sortedJSONKeys(typed) {
				child := typed[key]
				if values, ok := child.([]any); ok && len(values) > best.length && key != "truncated_files" && key != "notes" && key != "parents" {
					best = jsonArrayCandidate{object: typed, key: key, length: len(values)}
				}
				visit(child)
			}
		case []any:
			for _, child := range typed {
				visit(child)
			}
		}
	}
	visit(root)
	if best.object != nil && dropArrayTail(best.object, best.key, over, size, "") {
		return true
	}
	return trimLargestNamedString(root, over, map[string]bool{"content": true, "source": true, "body": true, "message": true, "note": true})
}

func trimLargestNamedString(root map[string]any, over int, names map[string]bool) bool {
	var bestObject map[string]any
	bestKey := ""
	bestLength := 0
	var visit func(any)
	visit = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for _, key := range sortedJSONKeys(typed) {
				child := typed[key]
				if text, ok := child.(string); ok && names[key] && len(text) > bestLength {
					bestObject, bestKey, bestLength = typed, key, len(text)
				}
				visit(child)
			}
		case []any:
			for _, child := range typed {
				visit(child)
			}
		}
	}
	visit(root)
	return bestObject != nil && trimStringField(bestObject, bestKey, over, true)
}

func sortedJSONKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
