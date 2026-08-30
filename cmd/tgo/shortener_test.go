package main

import (
	"net/url"
	"strings"
	"testing"
)

const sample = `ErrorDocument 404 /404.html

# main resources
RedirectTemp /home	https://tgo.example/
RedirectTemp /code  https://github.com/ramalho/tgo.li
RedirectTemp /up	HTTPS://Example.COM
RedirectTemp /22	https://go.dev/doc/effective_go	# extra field ignored
RedirectTemp /23	https://go.dev/doc/effective_go
RedirectTemp /
RedirectTemp /oops
Redirect /perm	https://example.com/
`

func TestSDigits(t *testing.T) {
	if len(SDigits) != 30 {
		t.Errorf("len(SDigits) = %d, want 30", len(SDigits))
	}
	for _, c := range "01ilu" {
		if strings.ContainsRune(SDigits, c) {
			t.Errorf("SDigits must not contain %q", c)
		}
	}
}

// normalizeCases covers the three rules that are applied and, just as
// importantly, the equivalences that are deliberately NOT applied.
var normalizeCases = []struct {
	name string
	in   string
	want string
}{
	// Rule A: the scheme is case-insensitive.
	{"uppercase scheme", "HTTPS://go.dev/Doc", "https://go.dev/Doc"},
	{"mixed scheme and host", "HtTpS://Go.Dev/Doc/Effective_Go", "https://go.dev/Doc/Effective_Go"},

	// Rule B: the host is case-insensitive; nothing else in the URL is.
	{"uppercase host", "https://Docs.Python.ORG/3/library/", "https://docs.python.org/3/library/"},
	{"userinfo keeps its case", "https://User@Example.COM/private", "https://User@example.com/private"},
	{"path keeps its case", "https://example.com/Path", "https://example.com/Path"},

	// Rule D: an absent path means "/" when there is an authority.
	{"bare domain", "https://dask.org", "https://dask.org/"},
	{"root path already there", "https://dask.org/", "https://dask.org/"},
	{"empty path with query", "https://dask.org?q=1", "https://dask.org/?q=1"},
	{"empty path with fragment", "https://dask.org#install", "https://dask.org/#install"},

	// Rule C is deliberately not implemented: the port is left as written.
	{"default https port kept", "https://example.com:443/a", "https://example.com:443/a"},
	{"default http port kept", "http://example.com:80/a", "http://example.com:80/a"},
	{"other port kept", "https://example.com:8443/a", "https://example.com:8443/a"},

	// Not applied: these can select a different resource.
	{"deep trailing slash kept", "https://docs.python.org/3/howto/", "https://docs.python.org/3/howto/"},
	{"fragment kept", "https://docs.python.org/3/x.html#attrs", "https://docs.python.org/3/x.html#attrs"},
	{"www kept", "https://www.pypy.org/", "https://www.pypy.org/"},
	{"query order kept", "https://example.com/s?b=2&a=1", "https://example.com/s?b=2&a=1"},
	{"encoded slash kept", "https://example.com/a%2Fb", "https://example.com/a%2Fb"},

	// Not applied: safe per RFC 3986 §6.2.2, but excluded on purpose.
	{"unreserved escape kept", "https://example.com/a%7Eb", "https://example.com/a%7Eb"},
	{"escape hex case kept", "https://example.com/caf%c3%a9/", "https://example.com/caf%c3%a9/"},
	{"dot segments kept", "https://example.com/3/../howto", "https://example.com/3/../howto"},
}

func TestNormalizeURL(t *testing.T) {
	for _, c := range normalizeCases {
		t.Run(c.name, func(t *testing.T) {
			u, err := normalizeURL(c.in)
			if err != nil {
				t.Fatalf("normalizeURL(%q): %s", c.in, err)
			}
			if got := u.String(); got != c.want {
				t.Errorf("normalizeURL(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// Normalizing an already normalized URL must be a no-op, otherwise the
// URLs stored in .htaccess would stop matching their own lookups.
func TestNormalizeURLIsIdempotent(t *testing.T) {
	for _, c := range normalizeCases {
		t.Run(c.name, func(t *testing.T) {
			u, err := normalizeURL(c.want)
			if err != nil {
				t.Fatalf("normalizeURL(%q): %s", c.want, err)
			}
			if got := u.String(); got != c.want {
				t.Errorf("normalizeURL(%q) = %q, want it unchanged", c.want, got)
			}
		})
	}
}

func TestNormalizeURLErrors(t *testing.T) {
	cases := []struct{ name, in string }{
		{"empty", ""},
		{"relative", "not-a-url"},
		{"no scheme", "go.dev/doc"},
		{"no host", "https:///doc"},
		{"scheme without authority", "mailto:luciano@example.com"},
		{"control character", "https://exa\x7fmple.com/"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if u, err := normalizeURL(c.in); err == nil {
				t.Errorf("normalizeURL(%q) = %q, want an error", c.in, u)
			}
		})
	}
}

func TestParseHtaccess(t *testing.T) {
	redir, targ, err := parseHtaccess(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	// redirects keeps the URL exactly as written in the file.
	wantRedir := map[string]string{
		"home": "https://tgo.example/",
		"code": "https://github.com/ramalho/tgo.li",
		"up":   "HTTPS://Example.COM",
		"22":   "https://go.dev/doc/effective_go",
		"23":   "https://go.dev/doc/effective_go",
	}
	if len(redir) != len(wantRedir) {
		t.Errorf("redirects = %v, want %v", redir, wantRedir)
	}
	for path, target := range wantRedir {
		if redir[path] != target {
			t.Errorf("redirects[%q] = %q, want %q", path, redir[path], target)
		}
	}
	// targets is keyed by the normalized URL, so a directive written before
	// normalization existed is still found.
	if got := targ["https://example.com/"]; got != "up" {
		t.Errorf("targets[example.com/] = %q, want %q", got, "up")
	}
	if _, found := targ["HTTPS://Example.COM"]; found {
		t.Error("targets should not be keyed by the raw URL")
	}
	// The same target twice: the first path wins.
	if got := targ["https://go.dev/doc/effective_go"]; got != "22" {
		t.Errorf("targets[effective_go] = %q, want %q", got, "22")
	}
	if len(targ) != 4 {
		t.Errorf("targets = %v, want 4 entries", targ)
	}
}

func TestNextPath(t *testing.T) {
	allTwo := redirects{}
	for _, a := range SDigits {
		for _, b := range SDigits {
			allTwo[string([]rune{a, b})] = "https://example.com/"
		}
	}
	cases := []struct {
		name string
		used redirects
		want string
	}{
		{"empty", redirects{}, "22"},
		{"two taken", redirects{"22": "u", "23": "u"}, "24"},
		{"gap in the middle", redirects{"22": "u", "24": "u"}, "23"},
		{"end of first digit", redirects{"22": "u", "23": "u", "24": "u", "25": "u",
			"26": "u", "27": "u", "28": "u", "29": "u", "2a": "u", "2b": "u", "2c": "u",
			"2d": "u", "2e": "u", "2f": "u", "2g": "u", "2h": "u", "2j": "u", "2k": "u",
			"2m": "u", "2n": "u", "2p": "u", "2q": "u", "2r": "u", "2s": "u", "2t": "u",
			"2v": "u", "2w": "u", "2x": "u", "2y": "u", "2z": "u"}, "32"},
		{"all two-character paths taken", allTwo, "222"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := nextPath(c.used)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("nextPath() = %q, want %q", got, c.want)
			}
		})
	}
}

// checkVariant asserts that toggle turns in into want (want == "" meaning no
// variant), that toggling twice returns the original -- so both spellings of a
// pair find each other -- and that the argument is left alone.
func checkVariant(t *testing.T, toggle func(*url.URL) *url.URL, in, want string) {
	t.Helper()
	u, err := normalizeURL(in)
	if err != nil {
		t.Fatal(err)
	}
	orig := u.String()
	v := toggle(u)
	if want == "" {
		if v != nil {
			t.Errorf("toggle(%q) = %q, want none", orig, v)
		}
		return
	}
	if v == nil {
		t.Fatalf("toggle(%q) = none, want %q", orig, want)
	}
	if got := v.String(); got != want {
		t.Errorf("toggle(%q) = %q, want %q", orig, got, want)
	}
	if back := toggle(v); back == nil || back.String() != orig {
		t.Errorf("toggle(toggle(%q)) = %v, want %q", orig, back, orig)
	}
	if got := u.String(); got != orig {
		t.Errorf("toggle modified its argument: %q, want %q", got, orig)
	}
}

func TestSlashVariant(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"add slash", "https://x.com/a", "https://x.com/a/"},
		{"remove slash", "https://x.com/a/", "https://x.com/a"},
		{"deep path", "https://x.com/a/b/", "https://x.com/a/b"},
		{"root has no variant", "https://x.com/", ""},
		{"bare host has no variant", "https://x.com", ""},
		{"slash goes on the path, not the query", "https://x.com/s?q=1", "https://x.com/s/?q=1"},
		{"slash goes on the path, not the fragment", "https://x.com/a/#f", "https://x.com/a#f"},
		{"encoded slash is not a separator", "https://x.com/a%2Fb", "https://x.com/a%2Fb/"},
		{"encoded slash stays encoded", "https://x.com/a%2Fb/", "https://x.com/a%2Fb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { checkVariant(t, slashVariant, c.in, c.want) })
	}
}

func TestWWWVariant(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"add www", "https://example.com/a", "https://www.example.com/a"},
		{"remove www", "https://www.example.com/a", "https://example.com/a"},
		{"uppercase host was already folded", "https://WWW.Example.COM/a", "https://example.com/a"},
		{"port is preserved", "https://example.com:8080/a", "https://www.example.com:8080/a"},
		{"port is preserved when removing", "https://www.example.com:8080/a", "https://example.com:8080/a"},
		{"subdomain is not www", "https://docs.example.com/a", "https://www.docs.example.com/a"},
		{"bare www. host has no variant", "https://www./a", ""},
		{"bare www. host with port has no variant", "https://www.:8080/a", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { checkVariant(t, wwwVariant, c.in, c.want) })
	}
}

func TestSchemeVariant(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"https to http", "https://example.com/a", "http://example.com/a"},
		{"http to https", "http://example.com/a", "https://example.com/a"},
		{"uppercase scheme was already folded", "HTTPS://example.com/a", "http://example.com/a"},
		{"other scheme has no variant", "ftp://example.com/a", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { checkVariant(t, schemeVariant, c.in, c.want) })
	}
}

func TestNearMisses(t *testing.T) {
	targ := targets{
		"https://example.com/a/":    "22",
		"https://www.example.com/a": "23",
		"http://example.com/a":      "24",
		"https://example.org/z":     "25",
	}
	cases := []struct {
		name string
		in   string
		want []nearMiss
	}{
		{"all three axes at once", "https://example.com/a", []nearMiss{
			{"22", "a trailing slash"},
			{"23", "the www. prefix"},
			{"24", "http vs https"},
		}},
		{"one axis", "https://example.com/a/x", nil},
		{"nothing nearby", "https://elsewhere.net/q", nil},
		{"exact match is not a near miss", "https://example.org/z", nil},
		{"two axes at once is too far", "http://www.example.com/a/", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u, err := normalizeURL(c.in)
			if err != nil {
				t.Fatal(err)
			}
			got := nearMisses(u, targ)
			if len(got) != len(c.want) {
				t.Fatalf("nearMisses(%q) = %v, want %v", c.in, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("nearMisses(%q)[%d] = %v, want %v", c.in, i, got[i], c.want[i])
				}
			}
		})
	}
}
