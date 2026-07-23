package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/workflow"
	"github.com/dgrieser/nickpit/mappings"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var reasoningEffortCompletions = []string{"max", "xhigh", "high", "medium", "low", "minimal", "none", "off"}

func registerRootCompletions(root *cobra.Command, a *app) {
	registerEnumCompletion(root, "output", []string{"markdown", "json", "raw"})
	registerEnumCompletion(root, "diff-format", []string{"git", "git-json"})
	registerEnumCompletion(root, "reasoning-effort", reasoningEffortCompletions)
	registerEnumCompletion(root, "small-reasoning-effort", reasoningEffortCompletions)
	registerEnumCompletion(root, "verify-drop-policy", []string{"none", "refuted-only", "refuted-and-unverified"})
	registerEnumCompletion(root, "priority-threshold", []string{"0", "1", "2", "3"})
	registerEnumCompletion(root, "disable-styleguide", append([]string{"all"}, mappings.StyleGuideOrder()...))
	registerEnumCompletion(root, "step", workflowStepCompletions())
	_ = root.RegisterFlagCompletionFunc("profile", func(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
		return filterCompletions(a.profileCompletions(), prefix), cobra.ShellCompDirectiveNoFileComp
	})

	_ = root.MarkPersistentFlagDirname("workdir")
	_ = root.MarkPersistentFlagDirname("session-dir")
	_ = root.MarkPersistentFlagFilename("config", "yaml", "yml")
	_ = root.MarkPersistentFlagFilename("spec", "yaml", "yml")
	_ = root.MarkPersistentFlagFilename("findings", "json")
	_ = root.MarkPersistentFlagFilename("styleguide")
}

func registerEnumCompletion(cmd *cobra.Command, flag string, values []string) {
	_ = cmd.RegisterFlagCompletionFunc(flag, func(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
		return filterCompletions(values, prefix), cobra.ShellCompDirectiveNoFileComp
	})
}

func filterCompletions(values []string, prefix string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		candidate := value
		if head, _, ok := strings.Cut(value, "\t"); ok {
			candidate = head
		}
		if strings.HasPrefix(candidate, prefix) {
			out = append(out, value)
		}
	}
	return out
}

func workflowStepCompletions() []string {
	steps := []string{
		workflow.StepCollectContext,
		workflow.StepVerify,
		workflow.StepDedupe,
		workflow.StepMerge,
		workflow.StepFinalize,
		workflow.StepVerdict,
		workflow.StepSummarize,
	}
	for _, vector := range workflow.ReviewVectorIDs {
		steps = append(steps,
			workflow.StepReviewPrefix+vector,
			workflow.StepExtractPrefix+vector,
			workflow.StepNudgePrefix+vector,
			workflow.StepVerifyPrefix+vector,
			workflow.StepDedupePrefix+vector,
		)
	}
	return steps
}

func (a *app) profileCompletions() []string {
	cfg := config.DefaultConfig()
	path := a.configPath
	if strings.TrimSpace(path) == "" {
		path = config.DefaultConfigPath
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(a.completionBaseDir(), path)
	}
	if data, err := os.ReadFile(path); err == nil {
		var local struct {
			Profiles map[string]yaml.Node `yaml:"profiles"`
		}
		if yaml.Unmarshal(data, &local) == nil {
			for name := range local.Profiles {
				if _, exists := cfg.Profiles[name]; !exists {
					cfg.Profiles[name] = config.Profile{}
				}
			}
		}
	}
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func registerGitRefCompletion(cmd *cobra.Command, flag string, a *app, includeCommits bool) {
	_ = cmd.RegisterFlagCompletionFunc(flag, func(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
		return filterCompletions(a.gitRefCompletions(includeCommits), prefix), cobra.ShellCompDirectiveNoFileComp
	})
}

func (a *app) gitRefCompletions(includeCommits bool) []string {
	base := a.completionBaseDir()
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", base, "for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes", "refs/tags")
	data, err := cmd.Output()
	if err != nil {
		return nil
	}
	seen := map[string]bool{"HEAD": true}
	values := []string{"HEAD"}
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, "/HEAD") || seen[line] {
			continue
		}
		seen[line] = true
		values = append(values, line)
	}
	if includeCommits {
		logCmd := exec.CommandContext(ctx, "git", "-C", base, "log", "-n", "50", "--pretty=format:%h%x09%s")
		if logData, logErr := logCmd.Output(); logErr == nil {
			for line := range strings.SplitSeq(strings.TrimSpace(string(logData)), "\n") {
				hash, _, _ := strings.Cut(line, "\t")
				if hash == "" || seen[hash] {
					continue
				}
				seen[hash] = true
				values = append(values, line)
			}
		}
	}
	return values
}

func registerRepoPathCompletion(cmd *cobra.Command, flag string, a *app, directoriesOnly bool) {
	_ = cmd.RegisterFlagCompletionFunc(flag, func(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
		return a.repoPathCompletions(prefix, directoriesOnly), cobra.ShellCompDirectiveNoFileComp
	})
}

func (a *app) repoPathCompletions(prefix string, directoriesOnly bool) []string {
	cleanPrefix := filepath.Clean(filepath.FromSlash(prefix))
	if prefix == "" {
		cleanPrefix = "."
	}
	if filepath.IsAbs(cleanPrefix) || cleanPrefix == ".." || strings.HasPrefix(cleanPrefix, ".."+string(filepath.Separator)) {
		return nil
	}
	dirPart := filepath.Dir(cleanPrefix)
	namePrefix := filepath.Base(cleanPrefix)
	if cleanPrefix == "." {
		dirPart, namePrefix = ".", ""
	} else if strings.HasSuffix(prefix, "/") || strings.HasSuffix(prefix, string(filepath.Separator)) {
		dirPart, namePrefix = cleanPrefix, ""
	}
	entries, err := os.ReadDir(filepath.Join(a.completionBaseDir(), dirPart))
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == ".git" || !strings.HasPrefix(entry.Name(), namePrefix) {
			continue
		}
		if directoriesOnly && !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(dirPart, entry.Name())
		if dirPart == "." {
			candidate = entry.Name()
		}
		candidate = filepath.ToSlash(candidate)
		if entry.IsDir() {
			candidate += "/"
		}
		out = append(out, candidate)
	}
	sort.Strings(out)
	return out
}

func (a *app) completionBaseDir() string {
	if strings.TrimSpace(a.workDir) != "" {
		if abs, err := filepath.Abs(a.workDir); err == nil {
			return abs
		}
		return a.workDir
	}
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
