// Command check reads an .htaccess file managed by tgo and visits every
// short URL it defines, reporting the HTTP status, short path, and long URL
// of any one whose final response is an error -- the way a bare RedirectTemp
// directive cannot: a directive is only proof the redirect was requested,
// not that the destination still exists.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// userAgent stands in for a browser: many sites -- Wikipedia and O'Reilly
// among them -- return 403 to Go's default User-Agent on sight, which would
// otherwise drown every real dead link in false positives.
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"

func main() {
	base := flag.String("base", "", "base URL to check short paths against (default: derived from the .htaccess file name, e.g. TGO.LI.htaccess -> https://tgo.li/)")
	workers := flag.Int("c", 20, "number of concurrent requests")
	perHost := flag.Int("per-host", 4, "maximum concurrent requests to any one destination host")
	timeout := flag.Duration("timeout", 15*time.Second, "per-request timeout")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [-base URL] [-c N] [-per-host N] [-timeout DURATION] FILE\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	client := &http.Client{Timeout: *timeout}
	if err := run(os.Stdout, flag.Arg(0), *base, client, *workers, *perHost); err != nil {
		fmt.Fprintf(os.Stderr, "check: %s\n", err)
		os.Exit(1)
	}
}

// redirect is one RedirectTemp directive: path is the short path, without
// its leading slash, and target is the long URL it redirects to.
type redirect struct {
	path   string
	target string
}

// run reads the RedirectTemp directives in file, visits each short URL --
// under base, or under the domain derived from file's name when base is ""
// -- and writes the problem found, the short path, and the long URL, one
// per line, as each visit comes back, for every one that did not end in a
// 2xx response. Lines therefore arrive in whatever order the requests
// finish, not file order, so a check of many URLs shows progress instead of
// going silent until the last one lands. A clean run writes nothing.
func run(out io.Writer, file, base string, client *http.Client, workers, perHost int) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	redirs, err := parseRedirects(f)
	if err != nil {
		return err
	}
	if base == "" {
		base = domainOf(file)
	}
	checkAll(out, base, redirs, client, workers, perHost)
	return nil
}

// parseRedirects reads every RedirectTemp directive in r, in the order they
// appear, ignoring every other line -- comments, ErrorDocument, blanks.
func parseRedirects(r io.Reader) ([]redirect, error) {
	var redirs []redirect
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 || fields[0] != "RedirectTemp" {
			continue
		}
		path := strings.TrimPrefix(fields[1], "/")
		if path == "" {
			continue
		}
		redirs = append(redirs, redirect{path, fields[2]})
	}
	return redirs, scanner.Err()
}

// domainOf derives the base URL to check from the .htaccess file's name: a
// file such as TGO.LI.htaccess manages tgo.li, so stripping the suffix and
// folding to lower case gives the domain.
func domainOf(file string) string {
	name := strings.TrimSuffix(filepath.Base(file), ".htaccess")
	return "https://" + strings.ToLower(name) + "/"
}

// checkAll visits base+path for every redirect, using up to workers requests
// at once -- no more than perHost of them landing on the same destination
// host at the same time, so a handful of short paths that all point at one
// site cannot swamp it and have its throttling read back as a dead link --
// and writes the status code, path, and target of each failure to out as
// soon as it is known. Writes are serialized, so lines from concurrent
// requests never interleave, but their order otherwise follows completion,
// not redirs.
func checkAll(out io.Writer, base string, redirs []redirect, client *http.Client, workers, perHost int) {
	sem := make(chan struct{}, workers)
	hosts := newHostLimiter(perHost)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, r := range redirs {
		wg.Add(1)
		sem <- struct{}{}
		go func(r redirect) {
			defer wg.Done()
			defer func() { <-sem }()
			if host := hostOf(r.target); host != "" {
				hosts.acquire(host)
				defer hosts.release(host)
			}
			if code := visit(client, base+r.path); code != "" {
				mu.Lock()
				fmt.Fprintf(out, "%-4s\t%s\t%s\n", code, r.path, r.target)
				mu.Unlock()
			}
		}(r)
	}
	wg.Wait()
}

// hostOf is the host of rawURL, or "" when rawURL does not parse as a URL
// with one -- in which case there is nothing to throttle against.
func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}

// hostLimiter hands out a concurrency slot per destination host, creating
// each host's slot on first use.
type hostLimiter struct {
	mu    sync.Mutex
	slots map[string]chan struct{}
	limit int
}

func newHostLimiter(limit int) *hostLimiter {
	return &hostLimiter{slots: make(map[string]chan struct{}), limit: limit}
}

func (h *hostLimiter) acquire(host string) {
	h.mu.Lock()
	s, ok := h.slots[host]
	if !ok {
		s = make(chan struct{}, h.limit)
		h.slots[host] = s
	}
	h.mu.Unlock()
	s <- struct{}{}
}

func (h *hostLimiter) release(host string) {
	h.mu.Lock()
	s := h.slots[host]
	h.mu.Unlock()
	<-s
}

// visit fetches target, following redirects the way a browser would, and
// classifies what it got back: the numeric status for a 4xx or 5xx, "T/O"
// for a timeout, or "???" for any other failure to get a response at all --
// a dropped connection, a DNS lookup that failed, too many redirects. It
// returns "" when the final response is 2xx.
func visit(client *http.Client, target string) string {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return "???"
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		if err, ok := err.(*url.Error); ok && err.Timeout() {
			return "T/O"
		}
		return "???"
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return strconv.Itoa(resp.StatusCode)
	}
	return ""
}
