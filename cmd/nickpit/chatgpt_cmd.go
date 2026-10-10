package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"time"

	"github.com/dgrieser/nickpit/internal/chatgpt"
	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/spf13/cobra"
)

// chatGPTTokens is the process-wide token source of every ChatGPT profile
// client, so the primary and any second client refresh through one session.
var chatGPTTokens = sync.OnceValues(func() (*chatgpt.LazySession, error) {
	store, err := chatgpt.DefaultStore()
	if err != nil {
		return nil, err
	}
	return &chatgpt.LazySession{Store: store, Provider: chatgpt.NewProvider()}, nil
})

// profileTokens is the credential source of profile's LLM endpoint: the
// stored ChatGPT sign-in for auth: chatgpt, the api_key otherwise.
func profileTokens(profile config.Profile) llm.TokenSource {
	if profile.Auth != config.AuthChatGPT {
		return llm.StaticToken(profile.APIKey)
	}
	tokens, err := chatGPTTokens()
	if err != nil {
		return failingTokenSource{err: err}
	}
	return tokens
}

// profileAPI is the protocol name profile resolves to, for fingerprints that
// must tell protocols apart; the default protocol stays empty so existing
// fingerprints keep hashing as before.
func profileAPI(profile config.Profile) string {
	if profile.API == config.APIChatCompletions {
		return ""
	}
	return profile.API
}

type failingTokenSource struct{ err error }

func (f failingTokenSource) Token(context.Context) (string, error) { return "", f.err }

// checkProfileAuth fails fast when a profile whose primary or @small model
// signs in with ChatGPT is about to run without a usable sign-in, instead of
// failing on the first model request.
func checkProfileAuth(profileName string, profile config.Profile) error {
	small := config.EffectiveSmallProfile(profile)
	var who string
	switch {
	case profile.Auth == config.AuthChatGPT:
		who = fmt.Sprintf("profile %q", profileName)
	case small.Auth == config.AuthChatGPT:
		who = fmt.Sprintf("the small model of profile %q", profileName)
	default:
		return nil
	}
	store, err := chatgpt.DefaultStore()
	if err != nil {
		return err
	}
	if _, err := chatgpt.NewSession(store, chatgpt.NewProvider()); err != nil {
		return fmt.Errorf("%s uses Sign in with ChatGPT: %w", who, err)
	}
	return nil
}

func (a *app) newChatGPTCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chatgpt",
		Short: "Sign in with ChatGPT to review with your ChatGPT plan",
		Long: "Sign in with ChatGPT lets nickpit use the models of your ChatGPT Plus or Pro plan " +
			"instead of an API key. Sign in once with `nickpit chatgpt login`, then select the " +
			"built-in `chatgpt` profile (--profile chatgpt) or set `auth: chatgpt` on a profile.\n\n" +
			"Credentials are stored with owner-only permissions under the user config directory; " +
			"set " + chatgpt.AuthFileEnv + " to use another file (for example one signed in on another machine).",
	}

	var port int
	var noBrowser, reconsent bool
	login := &cobra.Command{
		Use:   "login",
		Short: "Sign in with ChatGPT in the browser",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := chatgpt.DefaultStore()
			if err != nil {
				return err
			}
			previous, err := store.Load()
			if err != nil {
				return err
			}
			hostID, err := store.HostID()
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()
			ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()

			out := cmd.ErrOrStderr()
			opts := chatgpt.LoginOptions{
				Port:      port,
				Reconsent: reconsent,
				ShowURL: func(url string) {
					_, _ = fmt.Fprintf(out, "Open this URL to sign in with ChatGPT:\n\n  %s\n\nWaiting for the browser to return to http://127.0.0.1 ...\n", url)
				},
				// Keep the issued registration even if the code exchange fails,
				// so a retry reauthorizes instead of registering again.
				OnRegistration: store.SaveRegistration,
			}
			if !noBrowser {
				opts.OpenBrowser = openBrowser
			}
			creds, err := chatgpt.NewProvider().Login(ctx, previous, hostID, opts)
			if err != nil {
				return err
			}
			if err := store.Replace(creds); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(out, "\nSigned in as %s.\n", accountLabel(creds))
			if !creds.PlanUsage() {
				_, _ = fmt.Fprintf(out, "ChatGPT plan usage was not granted, so reviews cannot use this sign-in yet. Run `nickpit chatgpt login --reconsent` to grant it.\n")
				return nil
			}
			_, _ = fmt.Fprintf(out, "Use it with --profile chatgpt, or set `auth: chatgpt` on a profile. List your models with `nickpit chatgpt models`.\n")
			return nil
		},
	}
	login.Flags().IntVar(&port, "port", chatgpt.DefaultCallbackPort, "Loopback port of the sign-in callback (0 picks a free one)")
	login.Flags().BoolVar(&noBrowser, "no-browser", false, "Only print the sign-in URL instead of opening a browser")
	login.Flags().BoolVar(&reconsent, "reconsent", false, "Show the ChatGPT consent screen again, to grant plan usage")

	var forget bool
	logout := &cobra.Command{
		Use:   "logout",
		Short: "Sign out and revoke the ChatGPT session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := chatgpt.DefaultStore()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			result, err := chatgpt.Logout(ctx, store, chatgpt.NewProvider(), forget)
			if err != nil {
				return err
			}
			out := cmd.ErrOrStderr()
			if !result.SignedOut {
				_, _ = fmt.Fprintln(out, "Not signed in.")
				return nil
			}
			_, _ = fmt.Fprintln(out, "Signed out; local credentials removed.")
			if result.RevokeErr != nil {
				_, _ = fmt.Fprintf(out, "Remote sign-out could not be confirmed (%v); disconnect NickPit in ChatGPT settings if needed.\n", result.RevokeErr)
			}
			return nil
		},
	}
	logout.Flags().BoolVar(&forget, "forget", false, "Also forget the app registration and account, to sign in with another account")

	status := &cobra.Command{
		Use:   "status",
		Short: "Show the ChatGPT sign-in",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := chatgpt.DefaultStore()
			if err != nil {
				return err
			}
			creds, err := store.Load()
			if err != nil {
				return err
			}
			return writeChatGPTStatus(cmd.OutOrStdout(), store, creds)
		},
	}

	models := &cobra.Command{
		Use:   "models",
		Short: "List the models your ChatGPT plan can use",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := chatgpt.DefaultStore()
			if err != nil {
				return err
			}
			session, err := chatgpt.NewSession(store, chatgpt.NewProvider())
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
			defer cancel()
			list, err := session.ListModels(ctx)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, m := range list {
				if m.DisplayName != "" && m.DisplayName != m.Slug {
					_, _ = fmt.Fprintf(out, "%s\t%s\n", m.Slug, m.DisplayName)
				} else {
					_, _ = fmt.Fprintln(out, m.Slug)
				}
			}
			return nil
		},
	}

	cmd.AddCommand(login, logout, status, models)
	return cmd
}

func writeChatGPTStatus(out io.Writer, store *chatgpt.Store, creds *chatgpt.Credentials) error {
	_, _ = fmt.Fprintf(out, "Credentials: %s\n", store.AuthFile)
	if !creds.SignedIn() {
		_, _ = fmt.Fprintln(out, "Status:      not signed in (run `nickpit chatgpt login`)")
		return nil
	}
	_, _ = fmt.Fprintf(out, "Account:     %s\n", accountLabel(creds))
	planUsage := "granted"
	if !creds.PlanUsage() {
		planUsage = "not granted (run `nickpit chatgpt login --reconsent`)"
	}
	_, _ = fmt.Fprintf(out, "Plan usage:  %s\n", planUsage)
	expiry := "expired; renewed on next use"
	if time.Now().Before(creds.ExpiresAt) {
		expiry = "valid until " + creds.ExpiresAt.Local().Format(time.RFC1123)
	}
	_, _ = fmt.Fprintf(out, "Token:       %s\n", expiry)
	return nil
}

func accountLabel(creds *chatgpt.Credentials) string {
	switch {
	case creds.Email != "" && creds.Name != "":
		return fmt.Sprintf("%s <%s>", creds.Name, creds.Email)
	case creds.Email != "":
		return creds.Email
	case creds.Name != "":
		return creds.Name
	}
	return "your ChatGPT account"
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Stdout, cmd.Stderr = nil, nil
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
