package filetype

import (
	"path/filepath"
	"strings"
)

// DetectLanguage returns a syntax-oriented language label for a repo path.
func DetectLanguage(path string) string {
	normalized := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(normalized)

	if isHelmTemplate(normalized, base) {
		return "helm"
	}
	if base == "chart.lock" {
		return "yaml"
	}

	switch filepath.Ext(base) {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".mjs", ".cjs", ".ts", ".mts", ".cts", ".jsx", ".tsx", ".vue", ".svelte":
		return "nodejs"
	case ".rs":
		return "rust"
	case ".zig":
		return "zig"
	case ".java":
		return "java"
	case ".c":
		return "c"
	case ".h":
		return "c"
	case ".cc", ".cpp", ".cxx", ".c++", ".hh", ".hpp", ".hxx", ".h++":
		return "cpp"
	case ".yaml", ".yml":
		return "yaml"
	case ".json", ".jsonc", ".json5":
		return "json"
	case ".md", ".markdown", ".mdown", ".mkdn":
		return "markdown"
	case ".sh", ".bash", ".zsh", ".ksh":
		return "shell"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".cs":
		return "csharp"
	case ".kt", ".kts":
		return "kotlin"
	case ".swift":
		return "swift"
	case ".scala":
		return "scala"
	case ".sql":
		return "sql"
	case ".html", ".htm", ".cshtml":
		return "html"
	case ".css":
		return "css"
	case ".scss", ".sass":
		return "scss"
	case ".xml":
		return "xml"
	case ".toml":
		return "toml"
	case ".ini", ".cfg", ".conf":
		return "ini"
	case ".dockerfile", ".containerfile":
		return "dockerfile"
	default:
		switch base {
		case "dockerfile", "containerfile":
			return "dockerfile"
		case "makefile", "gnumakefile":
			return "makefile"
		default:
			return "text"
		}
	}
}

func isHelmTemplate(path, base string) bool {
	if strings.HasSuffix(base, ".tpl") {
		return true
	}
	return strings.Contains(path, "/templates/")
}
