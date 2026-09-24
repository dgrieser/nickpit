package config

import "github.com/dgrieser/nickpit/internal/model"

// The platform credentials stay explicit YAML fields (github_token,
// gitlab_token, gitlab_base_url, forgejo_token, forgejo_base_url) for the sake
// of the config file's shape; the
// accessors below are the one place that maps a platform to its fields, so
// the rest of NickPit addresses a platform by its ReviewMode alone.

// ForgeToken is the API token configured for the platform serving mode.
func (p Profile) ForgeToken(mode model.ReviewMode) string {
	switch mode {
	case model.ModeGitHub:
		return p.GitHubToken
	case model.ModeGitLab:
		return p.GitLabToken
	case model.ModeForgejo:
		return p.ForgejoToken
	default:
		return ""
	}
}

// ForgeBaseURL is the API base URL configured for the platform serving mode,
// empty for a platform with a fixed host.
func (p Profile) ForgeBaseURL(mode model.ReviewMode) string {
	switch mode {
	case model.ModeGitLab:
		return p.GitLabBaseURL
	case model.ModeForgejo:
		return p.ForgejoBaseURL
	default:
		return ""
	}
}

func (p *Profile) setForgeToken(mode model.ReviewMode, token string) {
	switch mode {
	case model.ModeGitHub:
		p.GitHubToken = token
	case model.ModeGitLab:
		p.GitLabToken = token
	case model.ModeForgejo:
		p.ForgejoToken = token
	}
}

func (p *Profile) setForgeBaseURL(mode model.ReviewMode, baseURL string) {
	switch mode {
	case model.ModeGitLab:
		p.GitLabBaseURL = baseURL
	case model.ModeForgejo:
		p.ForgejoBaseURL = baseURL
	}
}
