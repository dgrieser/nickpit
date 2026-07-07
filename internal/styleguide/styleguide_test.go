package styleguide

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

// srcs builds ungated (scalar) styleguide specs from source strings.
func srcs(sources ...string) []model.StyleGuideSpec {
	specs := make([]model.StyleGuideSpec, len(sources))
	for i, s := range sources {
		specs[i] = model.StyleGuideSpec{Source: s}
	}
	return specs
}

func writeGuide(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveFile(t *testing.T) {
	path := writeGuide(t, "team.md", "No TODO comments.\n")
	guides, err := Resolve(context.Background(), srcs(path), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(guides) != 1 {
		t.Fatalf("guides = %d, want 1", len(guides))
	}
	if guides[0].Language != path {
		t.Fatalf("language = %q, want spec", guides[0].Language)
	}
	want := "### Additional styleguide: " + path + "\n\nNo TODO comments."
	if guides[0].Content != want {
		t.Fatalf("content = %q, want %q", guides[0].Content, want)
	}
}

func TestResolveEmptySpecs(t *testing.T) {
	guides, err := Resolve(context.Background(), nil, "")
	if err != nil || guides != nil {
		t.Fatalf("guides, err = %#v, %v; want nil, nil", guides, err)
	}
}

func TestResolveMissingFile(t *testing.T) {
	spec := filepath.Join(t.TempDir(), "missing.md")
	_, err := Resolve(context.Background(), srcs(spec), "")
	if err == nil || !strings.Contains(err.Error(), spec) {
		t.Fatalf("error = %v, want to name %q", err, spec)
	}
}

func TestResolveDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := Resolve(context.Background(), srcs(dir), "")
	if err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("error = %v, want directory error", err)
	}
}

func TestResolveEmptyFile(t *testing.T) {
	path := writeGuide(t, "empty.md", "  \n\t\n")
	_, err := Resolve(context.Background(), srcs(path), "")
	if err == nil || !strings.Contains(err.Error(), "is empty") {
		t.Fatalf("error = %v, want empty error", err)
	}
}

func TestResolveOversizedFile(t *testing.T) {
	path := writeGuide(t, "big.md", strings.Repeat("a", MaxBytes+1))
	_, err := Resolve(context.Background(), srcs(path), "")
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v, want size error", err)
	}
}

func TestResolveBinaryFile(t *testing.T) {
	path := writeGuide(t, "bin.md", "rules\x00rules")
	_, err := Resolve(context.Background(), srcs(path), "")
	if err == nil || !strings.Contains(err.Error(), "is not text") {
		t.Fatalf("error = %v, want not-text error", err)
	}
}

func TestResolveNonRegularFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no /dev/null device file on windows")
	}
	_, err := Resolve(context.Background(), srcs("/dev/null"), "")
	if err == nil || !strings.Contains(err.Error(), "is not a regular file") {
		t.Fatalf("error = %v, want not-a-regular-file error", err)
	}
}

func TestResolveRelativePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("rules"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	guides, err := Resolve(context.Background(), srcs("guide.md"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(guides[0].Content, "rules") {
		t.Fatalf("content = %q", guides[0].Content)
	}
}

func TestResolveRelativePathAgainstBaseDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("workdir rules"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The process cwd is elsewhere; baseDir must anchor the relative spec.
	t.Chdir(t.TempDir())
	guides, err := Resolve(context.Background(), srcs("guide.md"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(guides[0].Content, "workdir rules") {
		t.Fatalf("content = %q", guides[0].Content)
	}
	if guides[0].Language != "guide.md" {
		t.Fatalf("language = %q, want spec as typed", guides[0].Language)
	}

	// Absolute specs ignore baseDir.
	absolute := writeGuide(t, "abs.md", "absolute rules")
	guides, err = Resolve(context.Background(), srcs(absolute), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(guides[0].Content, "absolute rules") {
		t.Fatalf("content = %q", guides[0].Content)
	}
}

func TestResolveURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prove Content-Type is ignored: common hosts serve markdown as
		// octet-stream.
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("Remote rules.\n"))
	}))
	defer server.Close()

	guides, err := Resolve(context.Background(), srcs(server.URL), "")
	if err != nil {
		t.Fatal(err)
	}
	want := "### Additional styleguide: " + server.URL + "\n\nRemote rules."
	if guides[0].Content != want {
		t.Fatalf("content = %q, want %q", guides[0].Content, want)
	}
}

func TestResolveURLFollowsRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Redirected rules."))
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer server.Close()

	guides, err := Resolve(context.Background(), srcs(server.URL), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(guides[0].Content, "Redirected rules.") {
		t.Fatalf("content = %q", guides[0].Content)
	}
}

func TestResolveURLNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no such guide", http.StatusNotFound)
	}))
	defer server.Close()

	_, err := Resolve(context.Background(), srcs(server.URL), "")
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") || !strings.Contains(err.Error(), server.URL) {
		t.Fatalf("error = %v, want HTTP 404 naming the URL", err)
	}
}

func TestResolveURLOversizedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", MaxBytes+1)))
	}))
	defer server.Close()

	_, err := Resolve(context.Background(), srcs(server.URL), "")
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v, want size error", err)
	}
}

func TestResolveURLUnreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	_, err := Resolve(context.Background(), srcs(url), "")
	if err == nil || !strings.Contains(err.Error(), url) {
		t.Fatalf("error = %v, want to name %q", err, url)
	}
}

func TestResolveOrderAndFailFast(t *testing.T) {
	first := writeGuide(t, "first.md", "First.")
	second := writeGuide(t, "second.md", "Second.")
	guides, err := Resolve(context.Background(), srcs(first, second), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(guides) != 2 || guides[0].Language != first || guides[1].Language != second {
		t.Fatalf("guides = %#v, want order preserved", guides)
	}

	missing := filepath.Join(t.TempDir(), "missing.md")
	if _, err := Resolve(context.Background(), srcs(first, missing, second), ""); err == nil {
		t.Fatal("want error when any spec fails")
	}
}
