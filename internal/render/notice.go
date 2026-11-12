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

// cc0 is the id of the public-domain dedication: a work under it carries no copyright notice.
const cc0 = "CC0-1.0"

// Text licences the catalog takes, as a notice names them.
var licenceLines = map[string]string{
	"CC-BY-4.0": "Licensed under CC BY 4.0 (https://creativecommons.org/licenses/by/4.0/).",
	cc0:         "Dedicated to the public domain under CC0 1.0 (https://creativecommons.org/publicdomain/zero/1.0/).",
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

// textLicence reads the bundle's licence expression: the line for the employee's text, and whether that text is
// dedicated to the public domain. Other ids of the expression are the licences of parts of its skills.
func textLicence(expr string) (line string, dedicated bool) {
	for _, id := range strings.Split(expr, " AND ") {
		id = strings.TrimSpace(id)
		if l, ok := licenceLines[id]; ok {
			return l, id == cc0
		}
	}
	if expr == "" {
		return "No licence is stated for this employee: do not copy or redistribute it.", false
	}
	return "Licensed under " + expr + ".", false
}

// own says whether the employee is the catalog's own: its text is harnsy's, not another publisher's.
func own(e *catalog.Employee) bool { return e.Entry.Publisher == ownPublisher }

// credit is the head of the notice in one line: who made the work, its licence, where it is, what was changed. It is
// also the last line of the persona file, so the credit survives when only that file is copied.
func credit(e *catalog.Employee, change string) string {
	licence, dedicated := textLicence(e.Doc.License)
	return fmt.Sprintf("%s %s by %s %s Source: %s. Changes: %s. Provided as is, without warranty of any kind (see the licence).",
		title(e), e.Entry.Version, author(e, dedicated), licence, sourceURL(e), change)
}

// author names who made the work, with the copyright notice of its first publication. A work dedicated to the public
// domain has no copyright notice: its author is named as a courtesy.
func author(e *catalog.Employee, dedicated bool) string {
	name, holder, where := publisherName(e), publisherName(e), ", published on hr.harnsy.dev."
	if own(e) {
		name, holder, where = ownAuthor, ownHolder, "."
	}
	if dedicated {
		return name + "."
	}
	if y := year(e); y != "" {
		holder = y + " " + holder
	}
	return name + ", © " + holder + where
}

func publisherName(e *catalog.Employee) string {
	if e.Entry.PublisherName != "" {
		return e.Entry.PublisherName
	}
	return e.Entry.Publisher
}

// hasSkillLicence says whether a skill of the employee carries its own LICENSE file.
func hasSkillLicence(e *catalog.Employee) bool {
	for path := range e.Files {
		if rest, ok := strings.CutPrefix(path, "skills/"); ok {
			if _, file, _ := strings.Cut(rest, "/"); file == "LICENSE" {
				return true
			}
		}
	}
	return false
}

// notice is the text that travels with an installed employee: the credit line, who stands behind the work, the
// publisher's note, the licence of parts of its skills, the works it is built on as the bundle lists them, and what
// the licence does not cover.
func notice(e *catalog.Employee, change string) string {
	var b strings.Builder
	b.WriteString(credit(e, change) + "\n")
	from, names := "the publisher", "The names and logos «harnsy» and «hr.harnsy.dev» are not covered by this licence."
	if own(e) {
		from = ownPublisher
		names = "The names and logos «harnsy» and «hr.harnsy.dev», and the picture of this employee (not included here), are not covered by this licence."
	} else {
		b.WriteString("\nhr.harnsy.dev lists this work; it is not made or endorsed by harnsy.\n")
	}
	if n := e.Entry.LicenseNote; n != "" {
		fmt.Fprintf(&b, "\nNote from %s: %s\n", from, n)
	}
	if hasSkillLicence(e) {
		b.WriteString("\nSome skills carry another licence for the parts adapted from other works or for their scripts (for example MIT): it is the `LICENSE` file in that skill's folder, and it applies to those parts")
		if e.Doc.License != "" {
			b.WriteString(" (the bundle's licence: " + e.Doc.License + ")")
		}
		b.WriteString(".\n")
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
	b.WriteString("\n" + names + "\n")
	return b.String()
}
