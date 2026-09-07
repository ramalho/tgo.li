package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// sortedLines splits s into its non-empty lines and sorts them, so output
// from concurrent requests can be compared regardless of completion order.
func sortedLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	sort.Strings(lines)
	return lines
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Only the paths whose final response is an error are reported -- bare,
// without the base URL in front -- and a working one, /ok here, stays
// silent. Lines can arrive in any order, since requests run concurrently.
func TestRunReportsErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
		case "/gone":
			w.WriteHeader(http.StatusNotFound)
		case "/broken":
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	file := writeFile(t, dir, "TEST.htaccess",
		"RedirectTemp /ok\thttps://example.com/ok\n"+
			"RedirectTemp /gone\thttps://example.com/gone\n"+
			"RedirectTemp /broken\thttps://example.com/broken\n")

	var out strings.Builder
	if err := run(&out, file, srv.URL+"/", srv.Client(), 4, 4); err != nil {
		t.Fatal(err)
	}
	want := sortedLines(
		"404 \tgone\thttps://example.com/gone\n" +
			"500 \tbroken\thttps://example.com/broken\n")
	if got := sortedLines(out.String()); !reflect.DeepEqual(got, want) {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// A request that never gets a response at all is reported as "???", since
// there is no status code to quote.
func TestRunReportsConnectionErrors(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, dir, "TEST.htaccess", "RedirectTemp /x\thttps://example.com/x\n")

	var out strings.Builder
	client := &http.Client{Timeout: 2 * time.Second}
	// Nothing listens on port 1: the connection is refused right away.
	if err := run(&out, file, "http://127.0.0.1:1/", client, 2, 2); err != nil {
		t.Fatal(err)
	}
	want := "??? \tx\thttps://example.com/x\n"
	if got := out.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// A request that times out is reported as "T/O", distinct from any other
// failure to get a response.
func TestRunReportsTimeouts(t *testing.T) {
	// The handler outlasts the client's timeout but still returns on its
	// own, so httptest.Server.Close -- which waits for it -- does not hang.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()

	dir := t.TempDir()
	file := writeFile(t, dir, "TEST.htaccess", "RedirectTemp /x\thttps://example.com/x\n")

	var out strings.Builder
	client := &http.Client{Timeout: 50 * time.Millisecond}
	if err := run(&out, file, srv.URL+"/", client, 1, 1); err != nil {
		t.Fatal(err)
	}
	want := "T/O \tx\thttps://example.com/x\n"
	if got := out.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// With no -base override, the domain to check comes from the file's name.
func TestRunUsesDomainFromFileName(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "https://")

	dir := t.TempDir()
	file := writeFile(t, dir, strings.ToUpper(host)+".htaccess",
		"RedirectTemp /x\thttps://example.com/x\n")

	var out strings.Builder
	if err := run(&out, file, "", srv.Client(), 1, 1); err != nil {
		t.Fatal(err)
	}
	want := "404 \tx\thttps://example.com/x\n"
	if got := out.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestHostOf(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"https://example.com/a", "example.com"},
		{"http://example.com:8080/a", "example.com:8080"},
		{"relative/path", ""},
	}
	for _, c := range cases {
		if got := hostOf(c.raw); got != c.want {
			t.Errorf("hostOf(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

// A limit of 1 never lets two acquires of the same host overlap, no matter
// how many goroutines race for it.
func TestHostLimiterSerializes(t *testing.T) {
	h := newHostLimiter(1)
	var active, maxActive int32
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.acquire("example.com")
			defer h.release("example.com")
			n := atomic.AddInt32(&active, 1)
			for {
				old := atomic.LoadInt32(&maxActive)
				if n <= old || atomic.CompareAndSwapInt32(&maxActive, old, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			atomic.AddInt32(&active, -1)
		}()
	}
	wg.Wait()
	if maxActive != 1 {
		t.Errorf("max concurrent acquires = %d, want 1", maxActive)
	}
}

// Different hosts do not wait on each other's slot.
func TestHostLimiterIsPerHost(t *testing.T) {
	h := newHostLimiter(1)
	h.acquire("a.example")
	done := make(chan struct{})
	go func() {
		h.acquire("b.example")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("acquire on a different host blocked")
	}
}

func TestDomainOf(t *testing.T) {
	cases := []struct{ file, want string }{
		{"TGO.LI.htaccess", "https://tgo.li/"},
		{"FPY.LI.htaccess", "https://fpy.li/"},
		{"/path/to/FPY.LI.htaccess", "https://fpy.li/"},
	}
	for _, c := range cases {
		if got := domainOf(c.file); got != c.want {
			t.Errorf("domainOf(%q) = %q, want %q", c.file, got, c.want)
		}
	}
}

func TestParseRedirects(t *testing.T) {
	const in = "ErrorDocument 404 /404.html\n" +
		"\n# comment\n" +
		"RedirectTemp /book\thttps://example.com/book\n" +
		"RedirectTemp /   https://example.com/root\n" + // path is "/": no directive
		"RedirectTemp /code https://example.com/code\n"

	got, err := parseRedirects(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := []redirect{
		{"book", "https://example.com/book"},
		{"code", "https://example.com/code"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseRedirects = %+v, want %+v", got, want)
	}
}
