package main

import (
	"image"
	"image/png"
	"os"
	"strings"
	"testing"

	"rsc.io/qr"
)

// decode reads name as a PNG, failing the test if it is not one.
func decode(t *testing.T, name string) image.Rectangle {
	t.Helper()
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("%s is not a PNG: %s", name, err)
	}
	return img.Bounds()
}

// wantBounds is the size of the QR code encoding url.
func wantBounds(t *testing.T, url string) image.Rectangle {
	t.Helper()
	code, err := qr.Encode(url, qr.M)
	if err != nil {
		t.Fatal(err)
	}
	return code.Image().Bounds()
}

func TestRunShortPath(t *testing.T) {
	t.Chdir(t.TempDir())
	var out strings.Builder

	if err := run(&out, "23"); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "https://tgo.li/23\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	if got, want := decode(t, "23.png"), wantBounds(t, "https://tgo.li/23"); got != want {
		t.Errorf("bounds = %v, want %v", got, want)
	}
}

// A dotted argument is a URL of its own: no tgo.li prefix is added.
func TestRunDottedArg(t *testing.T) {
	t.Chdir(t.TempDir())
	const arg = "gopl.io"
	var out strings.Builder

	if err := run(&out, arg); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), arg+"\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	if got, want := decode(t, arg+".png"), wantBounds(t, arg); got != want {
		t.Errorf("bounds = %v, want %v", got, want)
	}
}

// A full URL is encoded as given and named after its slug.
func TestRunURLArg(t *testing.T) {
	t.Chdir(t.TempDir())
	const arg = "https://gopl.io/ch1"
	var out strings.Builder

	if err := run(&out, arg); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), arg+"\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	if got, want := decode(t, "gopl.io-ch1.png"), wantBounds(t, arg); got != want {
		t.Errorf("bounds = %v, want %v", got, want)
	}
}

// A line of tgo output is accepted as it comes: the comment and the blanks
// around it are dropped.
func TestRunTgoLine(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, arg := range []string{
		"https://tgo.li/22  # new",
		"https://tgo.li/22  # existing",
		"  https://tgo.li/22\t",
	} {
		var out strings.Builder
		if err := run(&out, arg); err != nil {
			t.Fatalf("run(%q): %s", arg, err)
		}
		if got, want := out.String(), "https://tgo.li/22\n"; got != want {
			t.Errorf("run(%q): output = %q, want %q", arg, got, want)
		}
		if got, want := decode(t, "22.png"), wantBounds(t, "https://tgo.li/22"); got != want {
			t.Errorf("run(%q): bounds = %v, want %v", arg, got, want)
		}
	}
}

// An argument with nothing to encode is refused, before any PNG is written.
// A short URL on tgo.li is named after its path alone, as it was when
// "tgo -q" wrote these PNGs.
func TestRunShortURLKeepsPathName(t *testing.T) {
	t.Chdir(t.TempDir())
	const arg = "https://tgo.li/xy7"
	var out strings.Builder

	if err := run(&out, arg); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), arg+"\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	if got, want := decode(t, "xy7.png"), wantBounds(t, arg); got != want {
		t.Errorf("bounds = %v, want %v", got, want)
	}
}

func TestPngName(t *testing.T) {
	cases := []struct{ target, want string }{
		{"22", "22"},
		{"https://tgo.li/22", "22"},
		{"https://tgo.li/", "tgo.li"},     // no path to name it after
		{"http://tgo.li/22", "tgo.li-22"}, // only the canonical prefix counts
		{"https://gopl.io/ch1", "gopl.io-ch1"},
	}
	for _, c := range cases {
		if got := pngName(c.target); got != c.want {
			t.Errorf("pngName(%q) = %q, want %q", c.target, got, c.want)
		}
	}
}

// A # against the URL opens a fragment; only a # standing alone is a comment.
func TestClean(t *testing.T) {
	cases := []struct{ arg, want string }{
		{"https://tgo.li/22  # new", "https://tgo.li/22"},
		{"https://tgo.li/22 # existing", "https://tgo.li/22"},
		{"  https://tgo.li/22\t", "https://tgo.li/22"},
		{"https://example.com/a#top", "https://example.com/a#top"},
		{"https://example.com/a#top  # new", "https://example.com/a#top"},
		{"https://example.com/a #top", "https://example.com/a #top"}, // no blank after
		{"# new", "# new"}, // no blank before
		{"22", "22"},
	}
	for _, c := range cases {
		if got := clean(c.arg); got != c.want {
			t.Errorf("clean(%q) = %q, want %q", c.arg, got, c.want)
		}
	}
}

// A URL fragment survives, and names the PNG like any other URL character.
func TestRunKeepsFragment(t *testing.T) {
	t.Chdir(t.TempDir())
	const arg = "https://example.com/a#top"
	var out strings.Builder

	if err := run(&out, arg); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), arg+"\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	if got, want := decode(t, "example.com-a-top.png"), wantBounds(t, arg); got != want {
		t.Errorf("bounds = %v, want %v", got, want)
	}
}

func TestInput(t *testing.T) {
	cases := []struct {
		name  string
		stdin string
		args  []string
		want  string
	}{
		{"one argument", "", []string{"22"}, "22"},
		{"split by the shell", "", []string{"https://tgo.li/22", "#", "new"},
			"https://tgo.li/22 # new"},
		{"piped line", "https://tgo.li/22  # new\n", nil, "https://tgo.li/22  # new"},
		{"only the first line is read", "22\n23\n", nil, "22"},
		{"stdin ignored when there are arguments", "23\n", []string{"22"}, "22"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := input(strings.NewReader(c.stdin), c.args)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("input(%q, %q) = %q, want %q", c.stdin, c.args, got, c.want)
			}
		})
	}
}

// Empty stdin and no arguments is a usage error, not an empty QR code.
func TestInputNothing(t *testing.T) {
	if _, err := input(strings.NewReader(""), nil); err == nil {
		t.Error("input(empty, nil) = nil, want error")
	}
}

func TestRunUnnamableArg(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, arg := range []string{"", "https://", "..", "/", "   "} {
		if err := run(&strings.Builder{}, arg); err == nil {
			t.Errorf("run(%q) = nil, want error", arg)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct{ arg, want string }{
		{"23", "23"},
		{"gopl.io", "gopl.io"},
		{"https://gopl.io/ch1", "gopl.io-ch1"},
		{"https://gopl.io/", "gopl.io"},
		{"http://www.gopl.io:8080/a/b?q=1&r=2#top", "www.gopl.io-8080-a-b-q-1-r-2-top"},
		{"https://pt.wikipedia.org/wiki/Go_(linguagem)", "pt.wikipedia.org-wiki-Go_-linguagem"},
		{"https://tgo.li/ação", "tgo.li-a-o"},
		{"", ""},
		{"..", ""},
		{"/", ""},
	}
	for _, c := range cases {
		if got := slugify(c.arg); got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.arg, got, c.want)
		}
	}
}

func TestSlugifyLength(t *testing.T) {
	long := "https://gopl.io/" + strings.Repeat("x", 2*maxSlugLen)
	if got := len(slugify(long)); got != maxSlugLen {
		t.Errorf("len(slugify(long)) = %d, want %d", got, maxSlugLen)
	}
}
