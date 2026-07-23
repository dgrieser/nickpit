package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newCompletionCmd(root *cobra.Command) *cobra.Command {
	var install bool
	cmd := &cobra.Command{
		Use:                   "completion bash|zsh",
		Short:                 "Generate shell completion script",
		Args:                  cobra.ExactArgs(1),
		ValidArgs:             []string{"bash", "zsh"},
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if install {
				path, err := installCompletion(root, args[0])
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Installed %s completion to %s\n", args[0], path)
				return nil
			}
			return generateCompletion(root, args[0], cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&install, "install", false, "Install completion script for the current user")
	cmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return cmd.ValidArgs, cobra.ShellCompDirectiveNoFileComp
	}
	return cmd
}

func generateCompletion(root *cobra.Command, shell string, w io.Writer) error {
	switch shell {
	case "bash":
		return root.GenBashCompletion(w)
	case "zsh":
		return root.GenZshCompletion(w)
	default:
		return fmt.Errorf("unsupported shell %q: expected bash or zsh", shell)
	}
}

func installCompletion(root *cobra.Command, shell string) (string, error) {
	path, err := completionInstallPath(shell)
	if err != nil {
		return "", err
	}
	var script bytes.Buffer
	if err := generateCompletion(root, shell, &script); err != nil {
		return "", err
	}
	if err := writeCompletionFile(path, script.Bytes()); err != nil {
		return "", fmt.Errorf("installing %s completion: %w", shell, err)
	}
	return path, nil
}

func completionInstallPath(shell string) (string, error) {
	if shell == "bash" {
		if dir := strings.TrimSpace(os.Getenv("BASH_COMPLETION_USER_DIR")); dir != "" {
			return filepath.Join(dir, "nickpit"), nil
		}
	}
	dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving completion install directory: %w", err)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	switch shell {
	case "bash":
		return filepath.Join(dataHome, "bash-completion", "completions", "nickpit"), nil
	case "zsh":
		return filepath.Join(dataHome, "zsh", "site-functions", "_nickpit"), nil
	default:
		return "", fmt.Errorf("unsupported shell %q: expected bash or zsh", shell)
	}
}

func writeCompletionFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".nickpit-completion-*")
	if err != nil {
		return fmt.Errorf("creating temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setting file permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temporary file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	return nil
}
