package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/llm"
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
		ListingDescription: "with a repo-relative `path` to list all files in a folder (recursively).",
		Parameters: []CatalogParameter{
			{Name: "path", Type: "string", Description: "Repo-relative folder path; omit or pass an empty string to list the repo root", Example: `"<repo-relative folder>"`},
			{Name: "depth", Type: "integer", Description: "Optional traversal depth for nested folders; defaults to 1", Example: "int", Minimum: intPtr(1)},
		},
	},
	{
		Name:               "search",
		APIDescription:     "Search recursively inside repo-relative file or folder",
		ListingDescription: "with a repo-relative `path` and a `query` to search recursively for relevant matches",
		Note:               "Prefer `find_callers` over `search` when locating a function by name",
		Parameters: []CatalogParameter{
			{Name: "path", Type: "string", Description: "Repo-relative file or folder path; omit or pass an empty string to search from the repo root", Example: `"<repo-relative path>"`},
			{Name: "query", Type: "string", Description: "Search string to find", Example: `"<text>"`, Required: true},
			{Name: "context_lines", Type: "integer", Description: "Optional number of surrounding lines to include before and after each match; defaults to 5", Example: "int", Minimum: intPtr(0)},
			{Name: "max_results", Type: "integer", Description: "Optional maximum number of matches to return; omit or pass 0 for unlimited", Example: "int", Minimum: intPtr(0)},
			{Name: "case_sensitive", Type: "boolean", Description: "Optional case-sensitive match mode; defaults to false", Example: "bool"},
		},
	},
	{
		Name:               "find_callers",
		APIDescription:     "Resolve function by symbol name and return caller hierarchy including method bodies",
		ListingDescription: "with a `symbol`, optional repo-relative `path`, and optional `depth` to inspect which functions call a target function",
		Note:               "Prefer this over `search` when locating a function by name",
		Parameters:         callHierarchyParameters(),
	},
	{
		Name:               "find_callees",
		APIDescription:     "Resolve function by symbol name and return its callee hierarchy including method bodies",
		ListingDescription: "with a `symbol`, optional repo-relative `path`, and optional `depth` to inspect which functions a target function calls",
		Parameters:         callHierarchyParameters(),
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
		{Name: "path", Type: "string", Description: "Optional repo-relative file or folder path containing the function; omit or pass an empty string to search from the repo root", Example: `"<repo-relative path>"`},
		{Name: "depth", Type: "integer", Description: "Optional traversal depth for the call hierarchy; defaults to 10", Example: "int", Minimum: intPtr(1)},
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
	return fmt.Sprintf("%s\nNOTE: %s", entry.APIDescription, entry.Note)
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
