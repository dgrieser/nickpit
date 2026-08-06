package review

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/tokenestimate"
	"github.com/dgrieser/nickpit/internal/toollimits"
)

const toolResultTruncatedNote = "tool result exceeded configured item or size limits; narrow path, range, depth, or result count for more detail"

// minToolResultTokens is the floor for a single result's allowance. A prompt
// that already fills the context would otherwise leave nothing for any result
// in the batch, and a result the model cannot read at all is worse than a small
// one it can act on.
const minToolResultTokens = 256

// maxToolResultFloorTokens bounds what the floor above may add across one
// batch. Applied per result with no shared bound, a wide parallel batch against
// an already-full window granted every result the floor and overshot the
// context by their sum — the opposite of the shared remainder the batch is
// supposed to divide. Once the batch has spent this much on floors, later
// results take the allowance the remaining window actually affords.
const maxToolResultFloorTokens = 4 * minToolResultTokens

// limitToolResultBatch caps each result at a percentage of the context tokens
// still available when that result is appended. Processing the batch in order
// makes parallel results share the remaining window instead of each claiming
// the same allowance.
func limitToolResultBatch(meter *toolContextMeter, messages []llm.Message, tools []llm.ToolDefinition, schema []byte, batch []llm.Message, maxContextTokens, percent int) []llm.Message {
	if percent <= 0 || maxContextTokens <= 0 {
		return batch
	}
	limited := append([]llm.Message(nil), batch...)
	// The estimate is a byte count, so each appended result adds its own
	// encoding plus a separator. Tracking that incrementally keeps batch
	// limiting linear instead of re-encoding the whole transcript per result.
	contextBytes := meter.contextByteLen(messages, tools, schema)
	appended := len(messages)
	floorBudget := maxToolResultFloorTokens
	for i := range limited {
		used := tokenestimate.EstimateLen(contextBytes)
		remaining := max(maxContextTokens-used, 0)
		allowance := remaining * percent / 100
		if grant := min(max(minToolResultTokens-allowance, 0), floorBudget); grant > 0 {
			allowance += grant
			floorBudget -= grant
		}
		// A zero allowance means "no token cap" to limitToolResultJSON, so a
		// batch that has spent its floor budget asks for the smallest capped
		// result instead — the self-describing truncation marker.
		limited[i].Content = limitToolResultJSON(limited[i].Content, max(allowance, 1))
		if appended > 0 {
			contextBytes++ // the "," between array elements
		}
		contextBytes += jsonByteLen(limited[i])
		appended++
	}
	return limited
}

// releaseEmptiedToolResults hands back the dedup keys of every tool call whose
// result the context limiter reduced to a bare truncation marker. That marker
// tells the model to rerun with narrower arguments, so the calls it is being
// pointed at must not all answer already_requested — and for a search that was
// upgraded to a reference analysis, the reservation covers every spelling of
// find_references for that symbol, leaving it unreachable for the rest of the
// loop. inspect_file keeps its own reconciliation: a shortened file read still
// covers the lines that survived.
func releaseEmptiedToolResults(toolCalls []llm.ToolCall, raw, limited []llm.Message, state *toolRoundState) {
	for i, toolCall := range toolCalls {
		if i >= len(raw) || i >= len(limited) || raw[i].Content == limited[i].Content {
			continue
		}
		if toolResultLostPayload(limited[i].Content) {
			state.releaseToolCall(toolCall.ID)
		}
	}
}

// toolResultLostPayload reports whether limiting left nothing but the marker
// that says so — the fallback limitToolResultJSON returns when no reduction
// could fit the result in its allowance. A result that merely lost items still
// answers the call that produced it, so its keys stay taken.
func toolResultLostPayload(content string) bool {
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return false
	}
	if truncated, _ := payload["truncated"].(bool); !truncated {
		return false
	}
	for key := range payload {
		if key != "truncated" && key != "truncated_note" {
			return false
		}
	}
	return true
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

// toolContextMeter measures the encoded size of the request the tool results
// are appended to, reusing what it measured on earlier rounds. Encoding the
// whole transcript once per tool round made the agent loop quadratic in
// transcript size purely to divide a byte count by four, so each message is
// encoded once and re-encoded only when it changes.
//
// The zero value is usable, and a nil meter measures everything from scratch.
type toolContextMeter struct {
	lengths    []int
	signatures []toolMessageSignature
}

// toolMessageSignature is a cheap stand-in for a message's identity. The agent
// loop mostly appends, but it also rewinds the history on a JSON repair, so a
// cached length is only reused when the message at that index still looks the
// same. The content edges are sampled alongside its length: two different
// messages of the same length would otherwise share a signature, and comparing
// the whole content would cost as much as the encoding this avoids.
type toolMessageSignature struct {
	role       string
	toolCallID string
	content    int
	toolCalls  int
	head, tail string
}

const toolMessageSignatureSample = 24

func toolMessageSignatureOf(message llm.Message) toolMessageSignature {
	signature := toolMessageSignature{
		role:       message.Role,
		toolCallID: message.ToolCallID,
		content:    len(message.Content),
		toolCalls:  len(message.ToolCalls),
	}
	sample := min(len(message.Content), toolMessageSignatureSample)
	signature.head = message.Content[:sample]
	signature.tail = message.Content[len(message.Content)-sample:]
	return signature
}

// contextByteLen returns the encoded size of the request. The message slice is
// never nil so that appending an element grows the JSON array by exactly a
// separator plus the element — the same accounting the batch loop continues.
func (m *toolContextMeter) contextByteLen(messages []llm.Message, tools []llm.ToolDefinition, schema []byte) int {
	payload := struct {
		Messages []llm.Message        `json:"messages"`
		Tools    []llm.ToolDefinition `json:"tools,omitempty"`
		Schema   json.RawMessage      `json:"schema,omitempty"`
	}{
		Messages: []llm.Message{},
		Tools:    tools,
		Schema:   schema,
	}
	// Everything but the messages is bounded by the tool definitions and the
	// schema, so it is cheap to re-encode; the empty array it encodes here is
	// exactly two bytes wider than the separators counted below.
	total := jsonByteLen(payload)
	for i, message := range messages {
		if i > 0 {
			total++ // the "," between array elements
		}
		total += m.messageByteLen(i, message)
	}
	return total
}

func (m *toolContextMeter) messageByteLen(index int, message llm.Message) int {
	if m == nil {
		return jsonByteLen(message)
	}
	signature := toolMessageSignatureOf(message)
	if index < len(m.lengths) && m.signatures[index] == signature {
		return m.lengths[index]
	}
	length := jsonByteLen(message)
	for len(m.lengths) <= index {
		m.lengths = append(m.lengths, 0)
		m.signatures = append(m.signatures, toolMessageSignature{})
	}
	m.lengths[index], m.signatures[index] = length, signature
	return length
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

// applyToolItemLimits enforces the count limits that hold regardless of token
// budget. Both reference context lists are capped: with token capping disabled
// the payload would otherwise have no size bound at all.
func applyToolItemLimits(root map[string]any) bool {
	switch classifyToolResult(root) {
	case shapeReferences:
		return applyReferenceItemLimits(root)
	case shapeGroupedSearch:
		// One search can carry several reference analyses; the cap is per
		// analysis, exactly as it is for a lone one.
		changed := false
		for _, nested := range groupedStructuralPayloads(root) {
			if applyToolItemLimits(nested) {
				changed = true
			}
		}
		return changed
	}
	return false
}

func applyReferenceItemLimits(root map[string]any) bool {
	changed := false
	for _, field := range []string{"functions", "outside_functions"} {
		contexts := arrayField(root, field)
		if len(contexts) <= toollimits.MaxReferenceFunctions {
			continue
		}
		root[field] = contexts[:toollimits.MaxReferenceFunctions]
		addIntField(root, "omitted_contexts", len(contexts)-toollimits.MaxReferenceFunctions)
		markReferenceAnalysisIncomplete(root)
		changed = true
	}
	return changed
}

// markReferenceAnalysisIncomplete clears the completeness claim once contexts
// have been dropped. The reference counts still describe the whole analysis, so
// leaving `complete` true alongside a truncated context list reads as "these
// are all the uses" — the reader could conclude a symbol is never written when
// the writes were in the dropped tail.
func markReferenceAnalysisIncomplete(root map[string]any) {
	if _, ok := root["complete"]; ok {
		root["complete"] = false
	}
}

// reduceToolResult shrinks the payload by one step, preferring a reduction that
// suits the tool's shape. Every branch falls through to the generic reducer when
// its shape-specific reductions are exhausted; returning their result directly
// would strand an oversized payload whose typed fields are already empty, and
// the caller then discards it entirely.
func reduceToolResult(root map[string]any, over, size int) bool {
	switch classifyToolResult(root) {
	case shapeReferences:
		if dropArrayTail(root, "functions", over, size, "omitted_contexts") {
			markReferenceAnalysisIncomplete(root)
			return true
		}
		if dropArrayTail(root, "outside_functions", over, size, "omitted_contexts") {
			markReferenceAnalysisIncomplete(root)
			return true
		}
		if dropLiteralSearchResults(root, over, size) {
			return true
		}
		if target, ok := objectField(root, "target"); ok {
			if definition, ok := objectField(target, "definition"); ok {
				if trimStringField(definition, "content", over, true) {
					return true
				}
			}
		}
	case shapeCallHierarchy:
		if hierarchy, ok := root["root"].(map[string]any); ok {
			if trimDeepestHierarchyContent(hierarchy, over) {
				return true
			}
			if omitted := pruneDeepestHierarchyNodes(hierarchy, over, size); omitted > 0 {
				addIntField(root, "omitted_nodes", omitted)
				return true
			}
		}
		if dropLiteralSearchResults(root, over, size) {
			return true
		}
	case shapeGroupedSearch:
		if reduceGroupedSearchResult(root, over, size) {
			return true
		}
	case shapeSearchResults:
		if dropArrayTail(root, "results", over, size, "omitted_results") {
			root["result_count"] = len(arrayField(root, "results"))
			return true
		}
		if dropArrayTail(root, "truncated_files", over, size, "omitted_truncated_files") {
			return true
		}
	case shapeFileList:
		if dropArrayTail(root, "files", over, size, "omitted_files") {
			return true
		}
	case shapeHistory:
		if reduceHistoryResult(root, over, size) {
			return true
		}
	case shapeFileContent:
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

// reduceGroupedSearchResult shrinks the structural analyses a multi-declaration
// search returned, through the reducer that suits each one. Left to the generic
// reducer, the nested context lists were tail-dropped without an omitted count
// and without clearing the analysis's completeness claim — the one thing the
// reference branch exists to prevent.
func reduceGroupedSearchResult(root map[string]any, over, size int) bool {
	var largest map[string]any
	most := -1
	for _, nested := range groupedStructuralPayloads(root) {
		// Item counts stand in for encoded size here: measuring each nested
		// payload would mean re-encoding the whole result on every pass of the
		// shrink loop.
		if count := structuralPayloadItems(nested); count > most {
			largest, most = nested, count
		}
	}
	if largest != nil && reduceToolResult(largest, over, size) {
		return true
	}
	return dropLiteralSearchResults(root, over, size)
}

// dropLiteralSearchResults drops a tail of the literal matches a search result
// preserved next to its structural analysis, keeping the count that describes
// them true. Every structural shape can carry them, so every branch has to
// offer this step: the generic reducer finds the same array — it is usually the
// largest one — but leaves literal_result_count claiming matches the model can
// no longer see.
func dropLiteralSearchResults(root map[string]any, over, size int) bool {
	if !dropArrayTail(root, "literal_results", over, size, "omitted_literal_results") {
		return false
	}
	root["literal_result_count"] = len(arrayField(root, "literal_results"))
	return true
}

// groupedStructuralPayloads returns the call-hierarchy and reference analyses a
// grouped search result carries.
func groupedStructuralPayloads(root map[string]any) []map[string]any {
	var out []map[string]any
	for _, field := range []string{"reference_results", "call_hierarchies"} {
		for _, item := range arrayField(root, field) {
			if nested, ok := item.(map[string]any); ok {
				out = append(out, nested)
			}
		}
	}
	return out
}

func structuralPayloadItems(payload map[string]any) int {
	if hierarchy, ok := payload["root"].(map[string]any); ok {
		return countHierarchyNodes(hierarchy)
	}
	return len(arrayField(payload, "functions")) + len(arrayField(payload, "outside_functions"))
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

// toolResultShape names the decoded tool-result payloads the review layer has
// to tell apart. The tools answer with an untagged union of the retrieval
// result types, so the shape is recognized from the fields those types define.
// Recognizing it in one place is what keeps a new field named `target` or
// `results` on some unrelated result from silently rerouting pruning,
// summarization, or the history label.
//
// A search result carries its structural analysis at the top level and adds the
// literal matches it could not classify beside it, so it classifies as that
// analysis and never as shapeSearchResults. The literal matches are pruned by
// dropLiteralSearchResults, which every structural branch offers for exactly
// that reason.
type toolResultShape int

const (
	shapeUnknown       toolResultShape = iota
	shapeReferences                    // retrieval.ReferenceResult
	shapeCallHierarchy                 // retrieval.CallHierarchy
	shapeGroupedSearch                 // GroupedSearchResult
	shapeSearchResults                 // retrieval.SearchResults
	shapeFileList                      // list_files
	shapeFileContent                   // retrieval.FileContent / retrieval.FileSlice
	shapeHistory                       // git_log / git_show
)

func classifyToolResult(root map[string]any) toolResultShape {
	_, target := root["target"]
	_, functions := root["functions"]
	_, outside := root["outside_functions"]
	switch {
	case target && functions && outside:
		return shapeReferences
	case root["root"] != nil:
		return shapeCallHierarchy
	case root["reference_results"] != nil || root["call_hierarchies"] != nil:
		return shapeGroupedSearch
	case root["results"] != nil:
		return shapeSearchResults
	case root["files"] != nil:
		return shapeFileList
	case root["commits"] != nil:
		return shapeHistory
	case root["content"] != nil:
		return shapeFileContent
	}
	return shapeUnknown
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
