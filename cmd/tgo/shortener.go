package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// SDigits are the characters used in short paths.
// 0, 1, i, l and u are omitted to avoid confusion when reading URLs aloud
// or copying them from print.
const SDigits = "23456789abcdefghjkmnpqrstvwxyz"

// maxPathLen caps the search for an unused path.
const maxPathLen = 8

// redirects maps short paths to target URLs; targets maps them back.
type redirects map[string]string
type targets map[string]string

var errExhausted = errors.New("no short path available")

// normalizeURL parses raw and applies the three URL equivalences from
// RFC 3986 §6.2.2.1 and §6.2.3 that are true by definition, so that two
// spellings of the same resource get the same short path:
//
//	A. the scheme is case-insensitive
//	B. the host is case-insensitive
//	D. an absent path means "/" when there is an authority
//
// Everything else is left alone, including the port, trailing slashes on
// deeper paths, fragments, query order, and percent-encoding: those can
// select a different resource, and merging them would hand out a short URL
// pointing somewhere the caller never asked for.
func normalizeURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("not an absolute URL: %q", raw)
	}
	// A. url.Parse already lowercased the scheme.
	// B. The port is numeric and the userinfo lives in u.User, so lowercasing
	// the whole authority touches only the host.
	u.Host = strings.ToLower(u.Host)
	// D.
	if u.Path == "" {
		u.Path = "/"
	}
	return u, nil
}

// A nearMiss is an existing directive whose target differs from the new one
// by a single equivalence that tgo refuses to merge automatically: the two
// URLs are very often the same page, but nothing guarantees it, so tgo mints
// a separate short path and says what it noticed.
type nearMiss struct {
	path   string
	reason string
}

// variants are the toggles used to look for near misses. Each must be its own
// inverse, so both spellings of a pair find each other. A toggle returns nil
// when the URL has no counterpart on that axis.
var variants = []struct {
	reason string
	toggle func(*url.URL) *url.URL
}{
	{"a trailing slash", slashVariant},
	{"the www. prefix", wwwVariant},
	{"http vs https", schemeVariant},
}

// nearMisses reports every directive in targ one toggle away from u.
// Only one axis at a time is considered: a URL differing by both a scheme
// and a trailing slash is too far away to be worth guessing about.
func nearMisses(u *url.URL, targ targets) []nearMiss {
	var found []nearMiss
	for _, v := range variants {
		w := v.toggle(u)
		if w == nil {
			continue
		}
		if path, ok := targ[w.String()]; ok {
			found = append(found, nearMiss{path, v.reason})
		}
	}
	return found
}

// slashVariant returns u with the trailing slash of its path toggled, or nil
// when there is nothing to toggle. Rule D makes "/" the canonical root path,
// so a bare host has no variant to look for.
func slashVariant(u *url.URL) *url.URL {
	if u.Path == "" || u.Path == "/" {
		return nil
	}
	v := *u
	if strings.HasSuffix(v.Path, "/") {
		v.Path = strings.TrimSuffix(v.Path, "/")
		v.RawPath = strings.TrimSuffix(v.RawPath, "/")
	} else {
		v.Path += "/"
		if v.RawPath != "" {
			// A slash is never part of a percent-triplet, so appending one
			// keeps RawPath a valid encoding of Path -- unlike clearing it,
			// which would turn an encoded %2F into a real separator.
			v.RawPath += "/"
		}
	}
	return &v
}

// wwwVariant returns u with the www. prefix of its host toggled. Rule B has
// already lowercased the host, and any port follows the host, so the prefix
// can be toggled on u.Host directly.
func wwwVariant(u *url.URL) *url.URL {
	v := *u
	if rest, found := strings.CutPrefix(v.Host, "www."); found {
		if rest == "" || strings.HasPrefix(rest, ":") {
			return nil // "www." is the whole host
		}
		v.Host = rest
	} else {
		v.Host = "www." + v.Host
	}
	return &v
}

// schemeVariant returns u with http and https swapped, or nil for any other
// scheme.
func schemeVariant(u *url.URL) *url.URL {
	v := *u
	switch v.Scheme {
	case "http":
		v.Scheme = "https"
	case "https":
		v.Scheme = "http"
	default:
		return nil
	}
	return &v
}

// parseHtaccess reads RedirectTemp directives, ignoring every other line.
// Targets are keyed by their normalized form so that directives written
// before normalization existed are still found. When the same target URL
// appears more than once, the first path wins.
func parseHtaccess(r io.Reader) (redirects, targets, error) {
	redir := redirects{}
	targ := targets{}
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
		target := fields[2]
		if _, found := redir[path]; !found {
			redir[path] = target
		}
		key := target
		if u, err := normalizeURL(target); err == nil {
			key = u.String()
		}
		if _, found := targ[key]; !found {
			targ[key] = path
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	return redir, targ, nil
}

// nextPath returns the first path of length >= 2 that is not in used,
// generating them in odometer order: 22, 23, ..., zz, 222, ...
func nextPath(used redirects) (string, error) {
	for length := 2; length <= maxPathLen; length++ {
		idx := make([]int, length)
		digits := make([]byte, length)
		for {
			for i, d := range idx {
				digits[i] = SDigits[d]
			}
			path := string(digits)
			if _, found := used[path]; !found {
				return path, nil
			}
			pos := length - 1
			for ; pos >= 0; pos-- {
				idx[pos]++
				if idx[pos] < len(SDigits) {
					break
				}
				idx[pos] = 0
			}
			if pos < 0 { // wrapped around: every path of this length is used
				break
			}
		}
	}
	return "", errExhausted
}
