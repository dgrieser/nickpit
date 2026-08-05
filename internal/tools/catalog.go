package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/retrieval/goparser"
)

type catalogEntry struct {
	Name               string
	APIDescription     string
	ListingDescription string
	Note               string // optional
	Parameters         []CatalogParameter
}

type CatalogParameter struct {
	Name        string
	Type        string
	Description string
	Example     string
	Required    bool
	Minimum     *int
	Maximum     *int
}

type ErrorData struct {
	Code     string
	ToolName string
	Argument string
	Schema   string
	Message  string
}

type errorDefinition struct {
	Code    string
	Message string
}

var catalogDefinition = []catalogEntry{
	{
		Name:               "inspect_file",
		APIDescription:     "Retrieve content of repo-relative file",
		ListingDescription: "with a repo-relative `path` to retrieve the contents of a file",
		Parameters: []CatalogParameter{
			{Name: "path", Type: "string", Description: "Repo-relative file path", Example: `"<repo-relative path>"`, Required: true},
			{Name: "line_start", Type: "integer", Description: "Optional starting line number for partial file retrieval", Example: "int", Minimum: intPtr(1)},
			{Name: "line_end", Type: "integer", Description: "Optional ending line number for partial file retrieval", Example: "int", Minimum: intPtr(1)},
		},
	},
	{
		Name:               "list_files",
		APIDescription:     "List files of repo-relative folder",
		ListingDescription: "with a repo-relative `path` to list all files in a folder (recursively)",
		Parameters: []CatalogParameter{
			{Name: "path", Type: "string", Description: "Repo-relative folder path; omit or pass an empty string to list the repo root", Example: `"<repo-relative folder>"`},
			{Name: "depth", Type: "integer", Description: "Optional traversal depth for nested folders; defaults to 1", Example: "int", Minimum: intPtr(1)},
		},
	},
	{
		Name:               "search",
		APIDescription:     "Search recursively inside repo-relative file or folder for text or an exact line or block of code, returning each match as a `code_location` with exact line numbers",
		ListingDescription: "with a `query` (search text, or an exact line or block of code) and an optional repo-relative `path` to search recursively; every match is returned as a `code_location` with exact line numbers, the matching code and language",
		Note:               "Use this whenever you need to return a `code_location` in findings or suggestions: pass the exact line or block of code as `query` and copy a returned `code_location`. For symbols, prefer `find_callers` for upstream call paths, `find_callees` for downstream call paths, or `find_references` for all usage kinds",
		Parameters: []CatalogParameter{
			{Name: "path", Type: "string", Description: "Optional repo-relative file or folder path; omit or pass an empty string to search from the repo root", Example: `"<repo-relative path>"`},
			{Name: "query", Type: "string", Description: "Text or code to find: a single line matches as a substring; a multi-line block of code matches exactly, ignoring indentation and surrounding whitespace", Example: `"<text, or line(s) of code>"`, Required: true},
			{Name: "context_lines", Type: "integer", Description: "Optional number of surrounding lines to include before and after each match; defaults to 5 for a single-line query and 0 for a multi-line block", Example: "int", Minimum: intPtr(0)},
			{Name: "max_results", Type: "integer", Description: "Optional maximum number of matches to return; omit or pass 0 for unlimited", Example: "int", Minimum: intPtr(0)},
			{Name: "case_sensitive", Type: "boolean", Description: "Optional case-sensitive match mode; defaults to false", Example: "bool"},
		},
	},
	{
		Name:               "find_callers",
		APIDescription:     "Return functions that directly or recursively call a target function, organized as an upstream call hierarchy with function bodies",
		ListingDescription: "with a function `symbol`, optional declaration `path`, and optional `depth` to trace which functions invoke it",
		Note:               "Use for upstream execution and impact tracing. Only call relationships count; imports, assignments, and passing the function as a value are excluded. File types without structural analysis fall back to literal symbol search",
		Parameters:         callHierarchyParameters(),
	},
	{
		Name:               "find_callees",
		APIDescription:     "Return functions directly or recursively called by a target function, organized as a downstream call hierarchy with function bodies",
		ListingDescription: "with a function `symbol`, optional declaration `path`, and optional `depth` to trace what it invokes",
		Note:               "Use for understanding implementation flow and downstream dependencies. File types without structural analysis fall back to literal symbol search",
		Parameters:         callHierarchyParameters(),
	},
	{
		Name:               "find_references",
		APIDescription:     "Return a symbol definition and its repository-wide reads, writes, imports, aliases, calls, and other usages, grouped by enclosing function or top-level statement",
		ListingDescription: "with a `symbol` and optional declaration `path` to inspect all usage kinds; referenced functions are returned with their bodies",
		Note:               "Use for variables, constants, parameters, fields, imports, types, functions, and other named bindings. Results are flat usage contexts, not a recursive call hierarchy. Dynamic-language matches may be marked possible; large results may be truncated",
		Parameters: []CatalogParameter{
			{Name: "symbol", Type: "string", Description: "Symbol name to inspect", Example: `"<symbol name>"`, Required: true},
			{Name: "path", Type: "string", Description: "Optional repo-relative file or folder containing the declaration; does not limit where references are collected", Example: `"<repo-relative path>"`},
		},
	},
	{
		Name:               "git_log",
		APIDescription:     "List commits, newest first, with subject, body, author, author and commit dates, parents and the files each commit changed including added/deleted line counts, but without diff content",
		ListingDescription: "with optional `commit` (SHA of any length, ref, or range like `a..b`), `since`, `until`, `author`, `paths`, `message` filters to list commits with their metadata and changed files, without diff content",
		Note:               "History is limited to the reviewed checkout and may be truncated; a `shallow` flag and `note` in the result say so. Use `git_show` when you need the actual changes of a commit",
		Parameters: []CatalogParameter{
			{Name: "commit", Type: "string", Description: "Optional revision to list history from (SHA of any length, ref) or a range like \"a..b\"; defaults to HEAD", Example: `"<sha|ref|a..b>"`},
			{Name: "since", Type: "string", Description: "Optional lower bound on the commit date, e.g. \"2026-01-02\" or \"2 weeks ago\"; rebased or cherry-picked commits are selected by when they were rewritten, not authored", Example: `"<date>"`},
			{Name: "until", Type: "string", Description: "Optional upper bound on the commit date", Example: `"<date>"`},
			{Name: "author", Type: "string", Description: "Optional author name or email to match", Example: `"<author>"`},
			{Name: "paths", Type: "string", Description: "Optional comma-separated repo-relative paths; only commits touching them are listed", Example: `"<repo-relative paths>"`},
			{Name: "message", Type: "string", Description: "Optional text the commit message must contain", Example: `"<message text>"`},
			{Name: "message_regex", Type: "boolean", Description: "Optional flag to treat message and author as extended regular expressions instead of literal text; defaults to false", Example: "bool"},
			{Name: "case_sensitive", Type: "boolean", Description: "Optional case-sensitive matching for message and author; defaults to false", Example: "bool"},
			{Name: "limit", Type: "integer", Description: "Optional maximum number of commits to list; defaults to 20", Example: "int", Minimum: intPtr(1), Maximum: intPtr(200)},
		},
	},
	{
		Name:               "git_show",
		APIDescription:     "Retrieve the full diff of one commit or a commit range as one diff per commit, each with its commit message, author and date",
		ListingDescription: "with a `commit` (SHA of any length, ref, or range like `a..b`) and optional `paths` to retrieve each commit's message, author and full diff, one diff per commit",
		Note:               "Merge commits are shown as a combined diff, or against their first parent when the combined diff is empty; `diff_mode` per commit says which. Use `git_log` first when you do not know the commit yet",
		Parameters: []CatalogParameter{
			{Name: "commit", Type: "string", Description: "Revision to show (SHA of any length, ref) or a range like \"a..b\"", Example: `"<sha|ref|a..b>"`, Required: true},
			{Name: "to", Type: "string", Description: "Optional end revision; forms a range together with commit", Example: `"<sha|ref>"`},
			{Name: "paths", Type: "string", Description: "Optional comma-separated repo-relative paths; each diff is limited to them, and a commit that changed none of them is still returned with an empty diff and a note", Example: `"<repo-relative paths>"`},
			{Name: "max_commits", Type: "integer", Description: "Optional maximum number of commits of a range to return; defaults to 10", Example: "int", Minimum: intPtr(1), Maximum: intPtr(50)},
		},
	},
}

var errorDefinitions = map[string]errorDefinition{
	"retrieval_unavailable":  {Code: "retrieval_unavailable", Message: "retrieval is unavailable for this review"},
	"unsupported_tool":       {Code: "unsupported_tool", Message: "unsupported tool %q"},
	"missing_argument":       {Code: "missing_argument", Message: "missing required argument: %s; expected %s"},
	"already_requested_file": {Code: "already_requested_file", Message: "file contents were already provided for this review"},
	"already_requested_tool": {Code: "already_requested_tool", Message: "tool result was already provided for this review"},
	"encoding_failed":        {Code: "encoding_failed", Message: "failed to encode tool result"},
}

func intPtr(value int) *int {
	return &value
}

func callHierarchyParameters() []CatalogParameter {
	return []CatalogParameter{
		{Name: "symbol", Type: "string", Description: "Function name to inspect", Example: `"<function name>"`, Required: true},
		{Name: "path", Type: "string", Description: "Optional repo-relative file or folder containing the function declaration; does not limit where calls are collected", Example: `"<repo-relative path>"`},
		{Name: "depth", Type: "integer", Description: "Optional traversal depth for the call hierarchy; defaults to 10", Example: "int", Minimum: intPtr(1), Maximum: intPtr(goparser.MaxCallHierarchyDepth)},
	}
}

func Definitions(names ...string) ([]llm.ToolDefinition, error) {
	entries, err := selectEntries(names...)
	if err != nil {
		return nil, err
	}
	definitions := make([]llm.ToolDefinition, 0, len(entries))
	for _, entry := range entries {
		definitions = append(definitions, llm.ToolDefinition{
			Name:        entry.Name,
			Description: entry.apiDescription(),
			Parameters:  entry.parametersJSON(),
		})
	}
	return definitions, nil
}

func InstructionsListing(names ...string) (string, error) {
	entries, err := selectEntries(names...)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	for _, entry := range entries {
		builder.WriteString(entry.listingLine())
		builder.WriteByte('\n')
	}
	builder.WriteString("Tool results may be truncated to the configured size limit; when `truncated` is true, narrow the path, range, depth, or result count and retry.\n")
	return strings.TrimRight(builder.String(), "\n"), nil
}

func ArgumentSchema(name string) string {
	entry, ok := lookupEntry(name)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(entry.Parameters))
	for _, parameter := range entry.Parameters {
		suffix := ""
		if !parameter.Required {
			suffix = "?"
		}
		parts = append(parts, fmt.Sprintf("%q%s: %s", parameter.Name, suffix, parameter.Example))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func ErrorMessage(data ErrorData) string {
	definition, ok := errorDefinitions[data.Code]
	if !ok {
		return data.Message
	}
	switch data.Code {
	case "unsupported_tool":
		return fmt.Sprintf(definition.Message, data.ToolName)
	case "missing_argument":
		return fmt.Sprintf(definition.Message, data.Argument, data.Schema)
	default:
		return definition.Message
	}
}

func selectEntries(names ...string) ([]catalogEntry, error) {
	if len(names) == 0 {
		return append([]catalogEntry(nil), catalogDefinition...), nil
	}
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}
	selected := make([]catalogEntry, 0, len(names))
	for _, entry := range catalogDefinition {
		if _, ok := wanted[entry.Name]; ok {
			selected = append(selected, entry)
			delete(wanted, entry.Name)
		}
	}
	if len(wanted) > 0 {
		missing := make([]string, 0, len(wanted))
		for name := range wanted {
			missing = append(missing, name)
		}
		sort.Strings(missing)
		return nil, fmt.Errorf("unsupported tools: %s", strings.Join(missing, ", "))
	}
	return selected, nil
}

func lookupEntry(name string) (catalogEntry, bool) {
	for _, entry := range catalogDefinition {
		if entry.Name == name {
			return entry, true
		}
	}
	return catalogEntry{}, false
}

func (entry catalogEntry) apiDescription() string {
	if entry.Note == "" {
		return entry.APIDescription
	}
	return fmt.Sprintf("%s; NOTE: %s", entry.APIDescription, entry.Note)
}

func (entry catalogEntry) listingLine() string {
	line := fmt.Sprintf("- `%s` tool %s", entry.Name, entry.ListingDescription)
	if entry.Note != "" {
		line += fmt.Sprintf("\n  NOTE: %s", entry.Note)
	}
	return line
}

func (entry catalogEntry) parametersJSON() json.RawMessage {
	data, err := json.Marshal(entry.parametersSchema())
	if err != nil {
		panic(fmt.Sprintf("tools: marshaling tool schema for %s: %v", entry.Name, err))
	}
	return data
}

func (entry catalogEntry) parametersSchema() map[string]any {
	properties := make(map[string]any, len(entry.Parameters))
	required := make([]string, 0)
	for _, parameter := range entry.Parameters {
		property := map[string]any{
			"type":        parameter.Type,
			"description": parameter.Description,
		}
		if parameter.Minimum != nil {
			property["minimum"] = *parameter.Minimum
		}
		if parameter.Maximum != nil {
			property["maximum"] = *parameter.Maximum
		}
		properties[parameter.Name] = property
		if parameter.Required {
			required = append(required, parameter.Name)
		}
	}
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
