package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tgoWarn runs the command against file, returning stdout and stderr.
func tgoWarn(t *testing.T, file, url string) (string, string) {
	t.Helper()
	var out, warn strings.Builder
	if err := run(&out, &warn, file, url); err != nil {
		t.Fatalf("run(%q, %q): %s", file, url, err)
	}
	return out.String(), warn.String()
}

// tgo is tgoWarn for the cases that must not warn at all.
func tgo(t *testing.T, file, url string) string {
	t.Helper()
	out, warn := tgoWarn(t, file, url)
	if warn != "" {
		t.Errorf("unexpected warning for %s: %q", url, warn)
	}
	return out
}

func read(t *testing.T, file string) string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRunCreatesFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), defaultFile)
	const url = "https://go.dev/doc/effective_go"

	if got, want := tgo(t, file, url), "https://tgo.li/22  # new\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	want := header + "RedirectTemp /22\t" + url + "\n"
	if got := read(t, file); got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
}

func TestRunExistingURL(t *testing.T) {
	file := filepath.Join(t.TempDir(), defaultFile)
	const url = "https://go.dev/doc/effective_go"

	tgo(t, file, url)
	before := read(t, file)

	if got, want := tgo(t, file, url), "https://tgo.li/22  # existing\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	if after := read(t, file); after != before {
		t.Errorf("file changed: %q, want %q", after, before)
	}
}

func TestRunSecondURL(t *testing.T) {
	file := filepath.Join(t.TempDir(), defaultFile)
	const first = "https://go.dev/doc/effective_go"
	const second = "https://pkg.go.dev/net/url"

	tgo(t, file, first)
	if got, want := tgo(t, file, second), "https://tgo.li/23  # new\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	want := header +
		"RedirectTemp /22\t" + first + "\n" +
		"RedirectTemp /23\t" + second + "\n"
	if got := read(t, file); got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
}

func TestRunSkipsUsedPaths(t *testing.T) {
	file := filepath.Join(t.TempDir(), defaultFile)
	seed := "# seeded\nRedirectTemp /22\thttps://example.com/a\nRedirectTemp /23 https://example.com/b\n"
	if err := os.WriteFile(file, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, want := tgo(t, file, "https://example.com/c"), "https://tgo.li/24  # new\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	if got, want := tgo(t, file, "https://example.com/a"), "https://tgo.li/22  # existing\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	if got := read(t, file); !strings.HasPrefix(got, seed) {
		t.Errorf("file = %q, want it to start with the seeded content", got)
	}
}

func TestRunStoresNormalizedURL(t *testing.T) {
	file := filepath.Join(t.TempDir(), defaultFile)

	if got, want := tgo(t, file, "HTTPS://Dask.ORG"), "https://tgo.li/22  # new\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	want := header + "RedirectTemp /22\thttps://dask.org/\n"
	if got := read(t, file); got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
}

// Spellings that RFC 3986 declares equivalent must all find the first code.
func TestRunMatchesNormalizedVariants(t *testing.T) {
	variants := []string{
		"https://dask.org/",
		"https://dask.org",
		"https://DASK.org/",
		"HTTPS://dask.org",
		"https://Dask.Org",
	}
	file := filepath.Join(t.TempDir(), defaultFile)
	tgo(t, file, variants[0])
	before := read(t, file)

	for _, v := range variants {
		t.Run(v, func(t *testing.T) {
			if got, want := tgo(t, file, v), "https://tgo.li/22  # existing\n"; got != want {
				t.Errorf("output = %q, want %q", got, want)
			}
		})
	}
	if after := read(t, file); after != before {
		t.Errorf("file changed: %q, want %q", after, before)
	}
}

// URLs that only look alike must keep separate codes.
func TestRunKeepsDistinctURLsDistinct(t *testing.T) {
	distinct := []string{
		"https://docs.python.org/3/howto/",
		"https://docs.python.org/3/howto",
		"https://www.docs.python.org/3/howto/",
		"http://docs.python.org/3/howto/",
		"https://docs.python.org/3/howto/#intro",
		"https://docs.python.org:443/3/howto/",
		"https://docs.python.org/3/HOWTO/",
	}
	file := filepath.Join(t.TempDir(), defaultFile)
	seen := map[string]bool{}
	for _, u := range distinct {
		out, _ := tgoWarn(t, file, u) // the slash pair warns; that is its own test
		if !strings.HasSuffix(out, "  # new\n") {
			t.Errorf("%s: output = %q, want a new short URL", u, out)
		}
		if seen[out] {
			t.Errorf("%s: reused short URL %q", u, out)
		}
		seen[out] = true
	}
	if len(seen) != len(distinct) {
		t.Errorf("got %d short URLs, want %d", len(seen), len(distinct))
	}
}

func TestRunWarnsOnNearMiss(t *testing.T) {
	cases := []struct{ name, first, second, reason string }{
		{"slash added", "https://docs.python.org/3/howto",
			"https://docs.python.org/3/howto/", "a trailing slash"},
		{"slash removed", "https://docs.python.org/3/howto/",
			"https://docs.python.org/3/howto", "a trailing slash"},
		{"slash with query", "https://example.com/s?q=1",
			"https://example.com/s/?q=1", "a trailing slash"},
		{"slash with fragment", "https://example.com/a#top",
			"https://example.com/a/#top", "a trailing slash"},
		{"slash with encoded slash in path", "https://example.com/a%2Fb",
			"https://example.com/a%2Fb/", "a trailing slash"},
		{"www added", "https://example.com/a",
			"https://www.example.com/a", "the www. prefix"},
		{"www removed", "https://www.example.com/a",
			"https://example.com/a", "the www. prefix"},
		{"www on a bare host", "https://example.com",
			"https://www.example.com", "the www. prefix"},
		{"https to http", "https://example.com/a",
			"http://example.com/a", "http vs https"},
		{"http to https", "http://example.com/a",
			"https://example.com/a", "http vs https"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), defaultFile)
			tgo(t, file, c.first)

			out, warn := tgoWarn(t, file, c.second)
			if want := "https://tgo.li/23  # new\n"; out != want {
				t.Errorf("stdout = %q, want %q", out, want)
			}
			want := "note: /22 already redirects to " + normalized(t, c.first) +
				"\n\t(differs only by " + c.reason + ")\n"
			if warn != want {
				t.Errorf("stderr = %q, want %q", warn, want)
			}
			// The warning is advisory: both directives are still written.
			if got := read(t, file); !strings.Contains(got, "/23\t"+normalized(t, c.second)+"\n") {
				t.Errorf("file = %q, want it to contain the second target", got)
			}
		})
	}
}

func normalized(t *testing.T, raw string) string {
	t.Helper()
	u, err := normalizeURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.String()
}

// One URL can be a near miss of several existing ones at once.
func TestRunWarnsOnEveryAxis(t *testing.T) {
	file := filepath.Join(t.TempDir(), defaultFile)
	for _, u := range []string{
		"https://example.com/a/",
		"https://www.example.com/a",
		"http://example.com/a",
	} {
		tgoWarn(t, file, u) // these are near misses of each other too
	}

	out, warn := tgoWarn(t, file, "https://example.com/a")
	if want := "https://tgo.li/25  # new\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	want := "note: /22 already redirects to https://example.com/a/\n\t(differs only by a trailing slash)\n" +
		"note: /23 already redirects to https://www.example.com/a\n\t(differs only by the www. prefix)\n" +
		"note: /24 already redirects to http://example.com/a\n\t(differs only by http vs https)\n"
	if warn != want {
		t.Errorf("stderr = %q, want %q", warn, want)
	}
}

// The note quotes the URL as stored in the file, not the normalized lookup key.
func TestRunWarnsWithStoredURL(t *testing.T) {
	file := filepath.Join(t.TempDir(), defaultFile)
	seed := "RedirectTemp /22\tHTTPS://Example.COM/a/\n"
	if err := os.WriteFile(file, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	_, warn := tgoWarn(t, file, "https://example.com/a")
	want := "note: /22 already redirects to HTTPS://Example.COM/a/\n\t(differs only by a trailing slash)\n"
	if warn != want {
		t.Errorf("stderr = %q, want %q", warn, want)
	}
}

// Differences on more than one axis, or on no axis tgo checks, stay silent.
func TestRunDoesNotWarn(t *testing.T) {
	cases := []struct{ name, first, second string }{
		{"different path case", "https://example.com/a", "https://example.com/A"},
		{"different fragment", "https://example.com/a", "https://example.com/a#top"},
		{"different query", "https://example.com/a?q=1", "https://example.com/a?q=2"},
		{"different subdomain", "https://docs.example.com/a", "https://api.example.com/a"},
		{"unrelated", "https://example.com/a", "https://example.org/b"},
		{"root path has no slash variant", "https://example.com/", "https://example.org/"},
		{"scheme and slash at once", "https://example.com/a", "http://example.com/a/"},
		{"www and slash at once", "https://example.com/a", "https://www.example.com/a/"},
		{"www and scheme at once", "https://example.com/a", "http://www.example.com/a"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), defaultFile)
			tgo(t, file, c.first)
			tgo(t, file, c.second) // tgo fails the test if anything warns
		})
	}
}

func TestRunErrors(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"relative URL", "not-a-url"},
		{"no scheme", "go.dev/doc"},
		{"no host", "https:///doc"},
		{"tgo.li URL", "https://tgo.li/22"},
		{"www.tgo.li URL", "https://www.tgo.li/22"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, defaultFile)
			var out, warn strings.Builder
			if err := run(&out, &warn, file, c.url); err == nil {
				t.Errorf("run(%q) = nil, want an error", c.url)
			}
			if out.Len() != 0 || warn.Len() != 0 {
				t.Errorf("output = %q / %q, want none", out.String(), warn.String())
			}
			if _, err := os.Stat(file); !os.IsNotExist(err) {
				t.Errorf("%s should not have been created", file)
			}
		})
	}
}
