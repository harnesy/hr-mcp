package render

import (
	"fmt"
	"strings"

	"github.com/harnesy/hr-mcp/internal/catalog"
)

// The catalog's own publisher and how its works are credited.
const (
	ownPublisher = "harnsy"
	ownAuthor    = "harnsy (Pavel Buchnev)"
	ownHolder    = "Pavel Buchnev (harnsy)"
)

// Text licences the catalog takes, as a notice names them.
var licenceLines = map[string]string{
	"CC-BY-4.0": "Licensed under CC BY 4.0 (https://creativecommons.org/licenses/by/4.0/).",
	"CC0-1.0":   "Dedicated to the public domain under CC0 1.0 (https://creativecommons.org/publicdomain/zero/1.0/).",
}

// sourceURL is the employee's page on the catalog.
func sourceURL(e *catalog.Employee) string {
	if e.Entry.CVURL != "" {
		return e.Entry.CVURL
	}
	return "https://hr.harnsy.dev/e/" + e.Entry.ID
}

// year is the year the employee was first published: the feed's date, else the day its file was packed.
func year(e *catalog.Employee) string {
	for _, d := range []string{e.Entry.Published, e.Manifest.CreatedAt} {
		if len(d) >= 4 {
			return d[:4]
		}
	}
	return ""
}

// textLicence splits the bundle's licence expression into the line for its text and whether more ids stand beside it
// (the licences of scripts in its skills).
func textLicence(expr string) (line string, more bool) {
	ids := strings.Split(expr, " AND ")
	for _, id := range ids {
		if l, ok := licenceLines[strings.TrimSpace(id)]; ok {
			return l, len(ids) > 1
		}
	}
	if expr == "" {
		return "No licence is stated in the bundle.", false
	}
	return "Licensed under " + expr + ".", false
}

// credit is the head of the notice in one line: who made the work, its licence, where it is, what was changed. It is
// also the last line of the persona file, so the credit survives when only that file is copied.
func credit(e *catalog.Employee, change string) string {
	by := fmt.Sprintf("%s %s by %s, published on hr.harnsy.dev.", title(e), e.Entry.Version, publisherName(e))
	if e.Entry.Publisher == ownPublisher {
		by = fmt.Sprintf("%s %s by %s, © %s %s.", title(e), e.Entry.Version, ownAuthor, year(e), ownHolder)
	}
	licence, _ := textLicence(e.Doc.License)
	return fmt.Sprintf("%s %s Source: %s. Changes: %s. Provided as is, without warranty of any kind (see the licence).",
		by, licence, sourceURL(e), change)
}

func publisherName(e *catalog.Employee) string {
	if e.Entry.PublisherName != "" {
		return e.Entry.PublisherName
	}
	return e.Entry.Publisher
}

// notice is the text that travels with an installed employee: the credit line, the catalog's note on names, the
// licence of scripts, and the works it is built on, as the bundle lists them.
func notice(e *catalog.Employee, change string) string {
	var b strings.Builder
	b.WriteString(credit(e, change) + "\n")
	if n := e.Entry.LicenseNote; n != "" {
		b.WriteString("\n" + n + "\n")
	}
	if _, more := textLicence(e.Doc.License); more {
		fmt.Fprintf(&b, "\nScripts in a skill are under the licence in the `LICENSE` file in that skill's folder (the bundle's licence: %s).\n", e.Doc.License)
	}
	if len(e.Doc.Attribution) > 0 {
		b.WriteString("\nBuilt on:\n")
		for _, a := range e.Doc.Attribution {
			modified := "not modified"
			if a.Modified {
				modified = "modified"
			}
			fmt.Fprintf(&b, "- %s, %s, %s, %s\n", a.Name, a.URL, a.License, modified)
		}
	}
	b.WriteString("\nThe names «harnsy» and «hr.harnsy.dev» and the employee's picture are not covered by this licence.\n")
	return b.String()
}
