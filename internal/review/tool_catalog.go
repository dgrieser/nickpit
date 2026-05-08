package review

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/llm"
)

type toolCatalogEntry struct {
	Name               string
	APIDescription     string
	ListingDescription string
	Parameters         []toolCatalogParameter
}

type toolCatalogParameter struct {
	Name        string
	Type        string
	Description string
	Example     string
	Required    bool
	Minimum     *int
}

type toolErrorDefinition struct {
	Code    string
	Message string
}

var toolCatalogDefinition = []toolCatalogEntry{
	{
		Name:               "inspect_file",
		APIDescription:     "Retrieve content of repo-relative file",
		ListingDescription: "with a repo-relative `path` to retrieve the contents of a file.",
		Parameters: []toolCatalogParameter{
			{Name: "path", Type: "string", Description: "Repo-relative file path", Example: `"<repo-relative path>"`, Required: true},
			{Name: "line_start", Type: "integer", Description: "Optional starting line number for partial file retrieval", Example: "int", Minimum: toolIntPtr(1)},
			{Name: "line_end", Type: "integer", Description: "Optional ending line number for partial file retrieval", Example: "int", Minimum: toolIntPtr(1)},
		},
	},
	{
		Name:               "list_files",
		APIDescription:     "List files of repo-relative folder",
		ListingDescription: "with a repo-relative `path` to list all files in a folder (recursively).",
		Parameters: []toolCatalogParameter{
			{Name: "path", Type: "string", Description: "Repo-relative folder path; omit or pass an empty string to list the repo root", Example: `"<repo-relative folder>"`},
			{Name: "depth", Type: "integer", Description: "Optional traversal depth for nested folders; defaults to 1", Example: "int", Minimum: toolIntPtr(1)},
		},
	},
	{
		Name:               "search",
		APIDescription:     "Search recursively inside repo-relative file or folder",
		ListingDescription: "with a repo-relative `path` and a `query` to search recursively for relevant matches.",
		Parameters: []toolCatalogParameter{
			{Name: "path", Type: "string", Description: "Repo-relative file or folder path; omit or pass an empty string to search from the repo root", Example: `"<repo-relative path>"`},
			{Name: "query", Type: "string", Description: "Search string to find", Example: `"<text>"`, Required: true},
			{Name: "context_lines", Type: "integer", Description: "Optional number of surrounding lines to include before and after each match; defaults to 5", Example: "int", Minimum: toolIntPtr(0)},
			{Name: "max_results", Type: "integer", Description: "Optional maximum number of matches to return; omit or pass 0 for unlimited", Example: "int", Minimum: toolIntPtr(0)},
			{Name: "case_sensitive", Type: "boolean", Description: "Optional case-sensitive match mode; defaults to false", Example: "bool"},
		},
	},
	{
		Name:               "find_callers",
		APIDescription:     "Resolve function by symbol name and return caller hierarchy and method bodies",
		ListingDescription: "with a `symbol`, optional repo-relative `path`, and optional `depth` to inspect which functions call a target function across Go, Python, and Node.js/TypeScript code.",
		Parameters:         callHierarchyToolCatalogParameters(),
	},
	{
		Name:               "find_callees",
		APIDescription:     "Resolve function by symbol name and return its callee hierarchy and method bodies",
		ListingDescription: "with a `symbol`, optional repo-relative `path`, and optional `depth` to inspect which functions a target function calls across Go, Python, and Node.js/TypeScript code.",
		Parameters:         callHierarchyToolCatalogParameters(),
	},
}

var toolErrorDefinitions = map[string]toolErrorDefinition{
	"retrieval_unavailable":  {Code: "retrieval_unavailable", Message: "retrieval is unavailable for this review"},
	"unsupported_tool":       {Code: "unsupported_tool", Message: "unsupported tool %q"},
	"missing_argument":       {Code: "missing_argument", Message: "missing required argument: %s; expected %s"},
	"already_requested_file": {Code: "already_requested_file", Message: "file contents were already provided for this review"},
	"already_requested_tool": {Code: "already_requested_tool", Message: "tool result was already provided for this review"},
	"encoding_failed":        {Code: "encoding_failed", Message: "failed to encode tool result"},
}

func toolIntPtr(value int) *int {
	return &value
}

func callHierarchyToolCatalogParameters() []toolCatalogParameter {
	return []toolCatalogParameter{
		{Name: "symbol", Type: "string", Description: "Function name to inspect", Example: `"<function name>"`, Required: true},
		{Name: "path", Type: "string", Description: "Optional repo-relative file or folder path containing the function; omit or pass an empty string to search from the repo root", Example: `"<repo-relative path>"`},
		{Name: "depth", Type: "integer", Description: "Optional traversal depth for the call hierarchy; defaults to 10", Example: "int", Minimum: toolIntPtr(1)},
	}
}

func reviewerToolDefinitions() []llm.ToolDefinition {
	definitions := make([]llm.ToolDefinition, 0, len(toolCatalogDefinition))
	for _, entry := range toolCatalogDefinition {
		definitions = append(definitions, llm.ToolDefinition{
			Name:        entry.Name,
			Description: entry.APIDescription,
			Parameters:  entry.parametersJSON(),
		})
	}
	return definitions
}

func toolInstructionsListing() string {
	var builder strings.Builder
	for _, entry := range toolCatalogDefinition {
		fmt.Fprintf(&builder, "- `%s` tool %s\n", entry.Name, entry.ListingDescription)
	}
	return strings.TrimRight(builder.String(), "\n")
}

func toolArgumentSchema(name string) string {
	entry, ok := lookupToolCatalogEntry(name)
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

func toolErrorMessage(data toolErrorData) string {
	definition, ok := toolErrorDefinitions[data.Code]
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

func lookupToolCatalogEntry(name string) (toolCatalogEntry, bool) {
	for _, entry := range toolCatalogDefinition {
		if entry.Name == name {
			return entry, true
		}
	}
	return toolCatalogEntry{}, false
}

func (entry toolCatalogEntry) parametersJSON() json.RawMessage {
	data, err := json.Marshal(entry.parametersSchema())
	if err != nil {
		panic(fmt.Sprintf("review: marshaling tool schema for %s: %v", entry.Name, err))
	}
	return data
}

func (entry toolCatalogEntry) parametersSchema() map[string]any {
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
