package forge

import "testing"

func TestRemoteHost(t *testing.T) {
	cases := []struct {
		remote string
		host   string
	}{
		{"git@github.com:owner/repo.git", "github.com"},
		{"https://gitlab.example.com/grp/proj.git", "gitlab.example.com"},
		{"ssh://git@gitlab.example.com:29418/grp/proj.git", "gitlab.example.com"},
		{"gitlab.example.com:grp/proj.git", "gitlab.example.com"},
		{"", ""},
		{"not-a-remote", ""},
	}
	for _, c := range cases {
		if got := RemoteHost(c.remote); got != c.host {
			t.Fatalf("RemoteHost(%q) = %q, want %q", c.remote, got, c.host)
		}
	}
}

func TestSameHost(t *testing.T) {
	if !SameHost("git@gitlab.example.com:grp/proj.git", "https://gitlab.example.com/api/v4") {
		t.Fatal("the project's own host was rejected")
	}
	if !SameHost("https://GitLab.Example.com/grp/proj.git", "https://gitlab.example.com/api/v4") {
		t.Fatal("hosts compare case-insensitively")
	}
	if SameHost("git@gitlab.other.com:grp/proj.git", "https://gitlab.example.com/api/v4") {
		t.Fatal("a token would have gone to a foreign host")
	}
	if !SameHost("", "https://gitlab.example.com/api/v4") {
		t.Fatal("an unknown remote must not disable the configured host")
	}
	if SameHost("git@gitlab.example.com:grp/proj.git", "") {
		t.Fatal("no configured host matches nothing")
	}
}

func TestParseRequestID(t *testing.T) {
	if id, err := ParseRequestID("42", "number"); err != nil || id != 42 {
		t.Fatalf("ParseRequestID(42) = %d, %v", id, err)
	}
	for _, raw := range []string{"", "0", "-1", "x"} {
		if _, err := ParseRequestID(raw, "number"); err == nil {
			t.Fatalf("ParseRequestID(%q) accepted", raw)
		}
	}
}
