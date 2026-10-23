package toolchain

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/dgrieser/nickpit/internal/model"
)

func TestScanGoModKubernetes(t *testing.T) {
	body := `module example.com/op

go 1.24

require (
	k8s.io/api v0.33.2
	k8s.io/client-go v0.33.2
	sigs.k8s.io/controller-runtime v0.21.0
)
`
	fsys := fstest.MapFS{"go.mod": &fstest.MapFile{Data: []byte(body)}}
	got := scanGoModKubernetes(fsys)
	if len(got) != 2 {
		t.Fatalf("entries = %#v", got)
	}
	for _, entry := range got {
		if entry.Language != langKubernetes || entry.Version != "1.33" {
			t.Errorf("entry = %#v", entry)
		}
	}
}

func TestScanMakefileEnvtest(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{"ENVTEST_K8S_VERSION = 1.31.0\n", "1.31.0"},
		{"# Kubernetes version\nENVTEST_K8S_VERSION ?= 1.30\n", "1.30"},
		{"ENVTEST_K8S_VERSION ?= $(shell go list -m -f \"{{ .Version }}\" k8s.io/api)\n", ""},
	}
	for _, tc := range tests {
		fsys := fstest.MapFS{"Makefile": &fstest.MapFile{Data: []byte(tc.body)}}
		if got := findVersion(scanMakefileEnvtest(fsys), "Makefile", "ENVTEST_K8S_VERSION"); got != tc.want {
			t.Errorf("Makefile %q: got %q, want %q", tc.body, got, tc.want)
		}
	}
}

func TestSetupHelmVersions(t *testing.T) {
	content := `jobs:
  lint:
    steps:
      - name: Set up Helm
        uses: azure/setup-helm@v4
        with:
          version: v3.14.4
      - name: Other
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - uses: azure/setup-helm@v4.2.0
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
      - uses: some/other@v1
        with:
          version: 9.9.9
      - uses: "azure/setup-helm@v4"
        with:
          version: '4.0.1'
`
	got := setupHelmVersions(content)
	want := []string{"3.14.4", "4.0.1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("setupHelmVersions = %v, want %v", got, want)
	}
}

func TestHelmAndKubernetesSources(t *testing.T) {
	fsys := fstest.MapFS{
		".tool-versions": &fstest.MapFile{Data: []byte("helm 3.15.2\nkubectl 1.32.1\n")},
		".gitlab-ci.yml": &fstest.MapFile{Data: []byte("lint:\n  image: alpine/helm:3.16.1\ne2e:\n  script: kind create cluster --image kindest/node:v1.31.4\n")},
		".github/workflows/e2e.yaml": &fstest.MapFile{Data: []byte(`steps:
  - uses: helm/kind-action@v1
    with:
      node_image: kindest/node:v1.30.8
`)},
	}
	got := ScanFS(fsys, ctxWithChangedFiles("charts/app/Chart.yaml", "deploy/k8s/deployment.yaml"))
	checks := []struct{ language, source, field, want string }{
		{langHelm, ".tool-versions", "helm", "3.15.2"},
		{langKubernetes, ".tool-versions", "kubectl", "1.32.1"},
		{langHelm, ".gitlab-ci.yml", "image alpine/helm", "3.16.1"},
		{langKubernetes, ".gitlab-ci.yml", "image kindest/node", "1.31.4"},
		{langKubernetes, ".github/workflows/e2e.yaml", "kindest/node", "1.30.8"},
	}
	for _, c := range checks {
		found := false
		for _, entry := range got {
			if entry.Language == c.language && entry.Source == c.source && entry.Field == c.field && entry.Version == c.want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing %s %s %s=%s in %#v", c.language, c.source, c.field, c.want, got)
		}
	}
}

func TestScanDatabaseImages(t *testing.T) {
	fsys := fstest.MapFS{
		"docker-compose.yml": &fstest.MapFile{Data: []byte(`services:
  db:
    image: postgres:16-alpine
  cache:
    image: "bitnami/postgresql:15.4.0"
  app:
    environment:
      DB_HOST: mysql:3306
      DATABASE_URL: postgres://user:pw@db:5432/app
  legacy:
    image: mysql:8.0
  gis:
    image: postgis/postgis:17-3.5
`)},
		".gitlab-ci.yml": &fstest.MapFile{Data: []byte("test:\n  services:\n    - mysql:8.4\n    - name: postgres:14\n")},
	}
	got := scanDatabaseImages(fsys)
	want := []model.ToolchainVersion{
		{Language: langPostgreSQL, Source: "docker-compose.yml", Field: "image postgres", Version: "16-alpine"},
		{Language: langPostgreSQL, Source: "docker-compose.yml", Field: "image bitnami/postgresql", Version: "15.4.0"},
		{Language: langMySQL, Source: "docker-compose.yml", Field: "image mysql", Version: "8.0"},
		{Language: langPostgreSQL, Source: "docker-compose.yml", Field: "image postgis/postgis", Version: "17-3.5"},
		{Language: langMySQL, Source: ".gitlab-ci.yml", Field: "image mysql", Version: "8.4"},
		{Language: langPostgreSQL, Source: ".gitlab-ci.yml", Field: "image postgres", Version: "14"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scanDatabaseImages =\n%#v\nwant\n%#v", got, want)
	}
}

func TestScanFSReportsDatabaseImagesForSQLChanges(t *testing.T) {
	fsys := fstest.MapFS{
		"docker-compose.yml":       &fstest.MapFile{Data: []byte("services:\n  db:\n    image: mysql:8.4\n")},
		"migrations/002_users.sql": &fstest.MapFile{Data: []byte("ALTER TABLE users ADD COLUMN email varchar(255);\n")},
	}
	got := ScanFS(fsys, ctxWithChangedFiles("migrations/002_users.sql"))
	if findVersion(got, "docker-compose.yml", "image mysql") != "8.4" {
		t.Fatalf("SQL change should report the pinned MySQL image, got %#v", got)
	}
	for _, entry := range got {
		if entry.Unavailable {
			t.Fatalf("companions must not produce unavailable entries: %#v", got)
		}
	}
	if got := ScanFS(fsys, ctxWithChangedFiles("README.md")); len(got) != 0 {
		t.Fatalf("non-SQL change must not report database images, got %#v", got)
	}
}
