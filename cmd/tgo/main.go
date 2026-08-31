package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"rsc.io/qr"
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
	qrcode := flag.Bool("q", false, "write a QR code PNG named PATH.png")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [-f FILE] [-q] URL\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(os.Stdout, os.Stderr, *file, flag.Arg(0), *qrcode); err != nil {
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
// When qrcode is true, the short URL is also written as a PNG QR code,
// whether the short path is new or was already there.
func run(out, warn io.Writer, file, rawTarget string, qrcode bool) error {
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
		fmt.Fprintf(out, "existing: %s%s\n", baseURL, path)
		return reportQR(out, file, path, qrcode)
	}
	path, err := nextPath(redir)
	if err != nil {
		return err
	}
	if err := appendRedirect(file, fresh, path, target); err != nil {
		return err
	}
	fmt.Fprintf(out, "new: %s%s\n", baseURL, path)
	if err := reportQR(out, file, path, qrcode); err != nil {
		return err
	}
	for _, miss := range nearMisses(u, targ) {
		fmt.Fprintf(warn, "note: /%s already redirects to %s\n\t(differs only by %s)\n",
			miss.path, redir[miss.path], miss.reason)
	}
	return nil
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

// reportQR writes the QR code for path and names the file on out.
// It does nothing when qrcode is false.
func reportQR(out io.Writer, file, path string, qrcode bool) error {
	if !qrcode {
		return nil
	}
	name, err := writeQR(file, path)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "qrcode: %s\n", name)
	return nil
}

// writeQR encodes the short URL of path as a PNG beside file, so the codes
// travel with the site they point to, and returns the file name.
// An existing PNG is overwritten: the short URL it encodes cannot change.
func writeQR(file, path string) (string, error) {
	code, err := qr.Encode(baseURL+path, qr.M)
	if err != nil {
		return "", err
	}
	name := filepath.Join(filepath.Dir(file), path+".png")
	if err := os.WriteFile(name, code.PNG(), 0o644); err != nil {
		return "", err
	}
	return name, nil
}
