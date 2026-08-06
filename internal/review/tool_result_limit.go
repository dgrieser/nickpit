package review

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/tokenestimate"
	toolcatalog "github.com/dgrieser/nickpit/internal/tools"
)

const toolResultTruncatedNote = "tool result exceeded configured item or size limits; narrow path, range, depth, or result count for more detail"

// minToolResultTokens is the floor for a single result's allowance. A prompt
// that already fills the context would otherwise leave nothing for any result
// in the batch, and a result the model cannot read at all is worse than a small
// one it can act on.
const minToolResultTokens = 256

// limitToolResultBatch caps each result at a percentage of the context tokens
// still available when that result is appended. Processing the batch in order
// makes parallel results share the remaining window instead of each claiming
// the same allowance.
func limitToolResultBatch(messages []llm.Message, tools []llm.ToolDefinition, schema []byte, batch []llm.Message, maxContextTokens, percent int) []llm.Message {
	if percent <= 0 || maxContextTokens <= 0 {
		return batch
	}
	limited := append([]llm.Message(nil), batch...)
	// The estimate is a byte count, so each appended result adds its own
	// encoding plus a separator. Tracking that incrementally keeps batch
	// limiting linear instead of re-encoding the whole transcript per result.
	contextBytes := toolContextByteLen(messages, tools, schema)
	appended := len(messages)
	for i := range limited {
		used := tokenestimate.EstimateLen(contextBytes)
		remaining := max(maxContextTokens-used, 0)
		allowance := max(remaining*percent/100, minToolResultTokens)
		limited[i].Content = limitToolResultJSON(limited[i].Content, allowance)
		if appended > 0 {
			contextBytes++ // the "," between array elements
		}
		contextBytes += jsonByteLen(limited[i])
		appended++
	}
	return limited
}

// reconcileLimitedInspectFileCoverage makes deduplication reflect content the
// model actually received, not larger ranges fetched before context limiting.
func reconcileLimitedInspectFileCoverage(toolCalls []llm.ToolCall, raw, limited []llm.Message, state *toolRoundState) {
	for i, toolCall := range toolCalls {
		if toolCall.Name != "inspect_file" || i >= len(raw) || i >= len(limited) || raw[i].Content == limited[i].Content {
			continue
		}
		var args struct {
			Path      string `json:"path"`
			LineStart int    `json:"line_start"`
			LineEnd   int    `json:"line_end"`
		}
		if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
			continue
		}
		path := normalizeToolPath(strings.TrimSpace(args.Path))
		if path == "" {
			continue
		}
		state.mu.Lock()
		if args.LineStart <= 0 && args.LineEnd <= 0 {
			if content, ok := state.seenFiles[path]; ok {
				content.Truncated = true
				state.seenFiles[path] = content
			}
			state.mu.Unlock()
			continue
		}
		var original, visible struct {
			Start int `json:"start_line"`
			End   int `json:"end_line"`
		}
		_ = json.Unmarshal([]byte(raw[i].Content), &original)
		_ = json.Unmarshal([]byte(limited[i].Content), &visible)
		ranges := state.seenFileRanges[path]
		for j, covered := range ranges {
			if covered.Start == original.Start && covered.End == original.End {
				ranges = append(ranges[:j], ranges[j+1:]...)
				break
			}
		}
		if visible.Start > 0 && visible.End >= visible.Start {
			ranges = append(ranges, model.LineRange{Start: visible.Start, End: visible.End})
		}
		state.seenFileRanges[path] = ranges
		state.mu.Unlock()
	}
}

func estimateToolContextTokens(messages []llm.Message, tools []llm.ToolDefinition, schema []byte) int {
	return tokenestimate.EstimateLen(toolContextByteLen(messages, tools, schema))
}

// toolContextByteLen returns the encoded size of the request the tool results
// are appended to. The message slice is never nil so that appending an element
// grows the JSON array by exactly a separator plus the element.
func toolContextByteLen(messages []llm.Message, tools []llm.ToolDefinition, schema []byte) int {
	if messages == nil {
		messages = []llm.Message{}
	}
	payload := struct {
		Messages []llm.Message        `json:"messages"`
		Tools    []llm.ToolDefinition `json:"tools,omitempty"`
		Schema   json.RawMessage      `json:"schema,omitempty"`
	}{
		Messages: messages,
		Tools:    tools,
		Schema:   schema,
	}
	return jsonByteLen(payload)
}

func jsonByteLen(value any) int {
	encoded, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	return len(encoded)
}

// limitToolResultJSON applies tool-specific item limits and the profile-wide
// token limit to the final JSON sent back to the model.
func limitToolResultJSON(raw string, maxTokens int) string {
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
	if !changed && (maxTokens <= 0 || tokenestimate.Estimate(raw) <= maxTokens) {
		return raw
	}
	if changed {
		markToolResultTruncated(root)
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		return raw
	}
	if maxTokens <= 0 || tokenestimate.Estimate(string(encoded)) <= maxTokens {
		return string(encoded)
	}

	markToolResultTruncated(root)
	for range 10_000 {
		encoded, err = json.Marshal(root)
		if err != nil {
			return raw
		}
		estimated := tokenestimate.Estimate(string(encoded))
		if estimated <= maxTokens {
			return string(encoded)
		}
		over := max(1, len(encoded)*(estimated-maxTokens)/max(estimated, 1))
		if !reduceToolResult(root, over, len(encoded)) {
			break
		}
	}

	// Returned even when it exceeds the allowance: an unexplained empty object
	// tells the model nothing and it retries the same call, while overshooting
	// the cap by a marker costs a few dozen tokens.
	fallback := map[string]any{
		"truncated":      true,
		"truncated_note": "tool result exceeded its remaining-context token allowance and could not retain its original structure; rerun with narrower arguments",
	}
	encoded, err = json.Marshal(fallback)
	if err != nil {
		return raw
	}
	return string(encoded)
}

func applyToolItemLimits(root map[string]any) bool {
	if !isReferencePayload(root) {
		return false
	}
	functions := arrayField(root, "functions")
	if len(functions) <= toolcatalog.MaxReferenceFunctions {
		return false
	}
	omitted := len(functions) - toolcatalog.MaxReferenceFunctions
	root["functions"] = functions[:toolcatalog.MaxReferenceFunctions]
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
			if omitted := pruneDeepestHierarchyNodes(hierarchy, over, size); omitted > 0 {
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

// pruneDeepestHierarchyNodes drops a proportional tail of children from every
// node at the deepest level that still has any. Pruning one leaf per pass makes
// a wide hierarchy of small nodes take thousands of re-encodings to fit;
// pruning the whole level shrinks it geometrically instead.
func pruneDeepestHierarchyNodes(root map[string]any, over, size int) int {
	var deepest []map[string]any
	bestDepth := -1
	var walk func(map[string]any, int)
	walk = func(node map[string]any, depth int) {
		children := arrayField(node, "children")
		if len(children) > 0 {
			if depth > bestDepth {
				bestDepth, deepest = depth, nil
			}
			if depth == bestDepth {
				deepest = append(deepest, node)
			}
		}
		for _, child := range children {
			if childNode, ok := child.(map[string]any); ok {
				walk(childNode, depth+1)
			}
		}
	}
	walk(root, 0)
	omitted := 0
	for _, parent := range deepest {
		children := arrayField(parent, "children")
		drop := min(max(1, len(children)*max(1, over)/max(1, size)), len(children))
		for _, dropped := range children[len(children)-drop:] {
			omitted += countHierarchyNodes(dropped)
		}
		parent["children"] = children[:len(children)-drop]
	}
	return omitted
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
