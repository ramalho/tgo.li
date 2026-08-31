// Command qr writes the QR code of a URL as a PNG, taking the URL from its
// arguments or, with none, from stdin -- so a line of tgo output can be
// piped straight into it.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"rsc.io/qr"
)

const baseURL = "https://tgo.li/"

// htaccessFile is the file tgo manages; a bare short path is only encoded
// if it has a RedirectTemp directive there, so qr never mints a QR code for
// a path tgo.li does not actually redirect.
const htaccessFile = "TGO.LI.htaccess"

// maxSlugLen keeps the PNG name well under the 255-byte limit that file
// systems impose on a single name.
const maxSlugLen = 100

func main() {
	arg, err := input(os.Stdin, os.Args[1:])
	if err == nil {
		err = run(os.Stdout, arg)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
		os.Exit(1)
	}
}

// input is the line to encode: the arguments joined, so that a line of tgo
// output survives the shell splitting it -- qr $(tgo URL) -- or the first
// line read from r when there are none, for tgo URL | qr.
func input(r io.Reader, args []string) (string, error) {
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("nothing on stdin; usage: qr [PATH|URL]")
	}
	return scanner.Text(), nil
}

// run writes the QR code for arg to a PNG in the current directory and
// echoes the encoded URL to out. Any trailing comment and blanks are
// dropped first; a dot anywhere in what is left means it is a URL of its
// own, encoded as given, otherwise it is a tgo.li short path, and must
// already have a RedirectTemp directive in htaccessFile -- no PNG is
// written for a path tgo.li would not actually redirect.
// An existing PNG is overwritten: the URL it encodes cannot change.
func run(out io.Writer, arg string) error {
	target := clean(arg)
	url := baseURL + target
	if strings.Contains(target, ".") {
		url = target
	} else if found, err := shortPathExists(htaccessFile, target); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("no such tgo.li short path: /%s (not found in %s)", target, htaccessFile)
	}
	name := pngName(target)
	if name == "" {
		return fmt.Errorf("nothing to encode in %q", arg)
	}
	code, err := qr.Encode(url, qr.M)
	if err != nil {
		return err
	}
	if err := os.WriteFile(name+".png", code.PNG(), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(out, url)
	return nil
}

// shortPathExists reports whether file has a RedirectTemp directive for
// path. A missing file means no path is defined yet.
func shortPathExists(file, path string) (bool, error) {
	f, err := os.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || fields[0] != "RedirectTemp" {
			continue
		}
		if strings.TrimPrefix(fields[1], "/") == path {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// pngName is the name, extension aside, of the file holding the QR code of
// target. A tgo.li short URL is named after its path alone -- 22.png -- as
// it was when "tgo -q" wrote these codes.
func pngName(target string) string {
	if path, found := strings.CutPrefix(target, baseURL); found && path != "" {
		return slugify(path)
	}
	return slugify(target)
}

// clean drops a comment and any surrounding blanks, so a line of tgo output
// -- "https://tgo.li/22  # new" -- can be handed to qr as it comes.
func clean(arg string) string {
	if i := comment(arg); i >= 0 {
		arg = arg[:i]
	}
	return strings.TrimSpace(arg)
}

// comment returns the index of the # opening a comment, or -1 when there is
// none. Only a # with a blank on each side counts, the shape tgo writes: a #
// against the URL opens a fragment and stays.
func comment(s string) int {
	for i, r := range s {
		if r != '#' {
			continue
		}
		before, _ := utf8.DecodeLastRuneInString(s[:i])
		after, _ := utf8.DecodeRuneInString(s[i+1:])
		if unicode.IsSpace(before) && unicode.IsSpace(after) {
			return i
		}
	}
	return -1
}

// slugify turns arg into a single file name: the scheme goes, and every run
// of characters a name cannot hold -- the slashes above all -- becomes one
// dash, so https://gopl.io/ch1 is written to gopl.io-ch1.png.
// A short path is already a legal name and comes back unchanged.
// Letter case is kept: URL paths are case-sensitive, and folding them would
// name two different pages the same file.
func slugify(arg string) string {
	if i := strings.Index(arg, "://"); i >= 0 {
		arg = arg[i+len("://"):]
	}
	var b strings.Builder
	dash := false // a run of unusable characters is pending
	for _, r := range arg {
		if isNameRune(r) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
			continue
		}
		dash = true
	}
	slug := b.String()
	if len(slug) > maxSlugLen {
		slug = slug[:maxSlugLen]
	}
	// A leading dot would hide the PNG, and "." or ".." is not a name at all.
	return strings.Trim(slug, ".-_")
}

// isNameRune reports whether r may appear in the slug. The set is
// deliberately narrow -- ASCII only -- so the name survives every file
// system, shell and web server the PNG may travel through.
func isNameRune(r rune) bool {
	switch {
	case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9':
		return true
	}
	return r == '.' || r == '-' || r == '_'
}
