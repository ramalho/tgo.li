package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	// defaultFile is the .htaccess file managed by this command,
	// deployed at the root of the tgo.li site renamed to .htaccess.
	defaultFile = "TGO.LI.htaccess"
	baseURL     = "https://tgo.li/"
	baseDomain  = "tgo.li"
	header      = "# TGO.LI redirects — managed by the tgo command\n"
)

func main() {
	file := flag.String("f", defaultFile, "path to the .htaccess file")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [-f FILE] URL\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(os.Stdout, os.Stderr, *file, flag.Arg(0)); err != nil {
		fmt.Fprintf(os.Stderr, "tgo: %s\n", err)
		os.Exit(1)
	}
}

// run looks up target in file, appending a new RedirectTemp directive
// if the URL is not there yet, and reports the short URL to out.
// The URL is stored and matched in its normalized form; a target that
// differs from an existing one by a single equivalence tgo will not merge
// gets its own short path plus a note on warn, because the two URLs may be
// different resources.
func run(out, warn io.Writer, file, rawTarget string) error {
	u, err := normalizeURL(rawTarget)
	if err != nil {
		return err
	}
	if host := strings.TrimPrefix(u.Hostname(), "www."); host == baseDomain {
		return fmt.Errorf("%q is already a %s URL", rawTarget, baseDomain)
	}
	target := u.String()

	data, err := os.ReadFile(file)
	fresh := false
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		fresh = true
	}
	redir, targ, err := parseHtaccess(strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	if path, found := targ[target]; found {
		report(out, path, "existing")
		return nil
	}
	path, err := nextPath(redir)
	if err != nil {
		return err
	}
	if err := appendRedirect(file, fresh, path, target); err != nil {
		return err
	}
	report(out, path, "new")
	for _, miss := range nearMisses(u, targ) {
		fmt.Fprintf(warn, "note: /%s already redirects to %s\n\t(differs only by %s)\n",
			miss.path, redir[miss.path], miss.reason)
	}
	return nil
}

// report writes the short URL of path to out, followed by label as a
// comment. The qr command drops everything from the # on, so the line can
// be piped straight into it.
func report(out io.Writer, path, label string) {
	fmt.Fprintf(out, "%s%s  # %s\n", baseURL, path, label)
}

func appendRedirect(file string, fresh bool, path, target string) error {
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if fresh {
		if _, err := f.WriteString(header); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(f, "RedirectTemp /%s\t%s\n", path, target); err != nil {
		return err
	}
	return f.Close()
}
