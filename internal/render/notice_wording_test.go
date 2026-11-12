package render

import (
	"strings"
	"testing"

	"github.com/harnesy/hr-mcp/internal/catalog"
)

// worker is a made-up employee with the fields a notice reads.
func worker(publisher, publisherName, licence string) *catalog.Employee {
	return &catalog.Employee{
		Entry: catalog.Entry{ID: publisher + "/sample", Version: "1.0.0", Title: "Sample", Publisher: publisher,
			PublisherName: publisherName, Published: "2026-03-04"},
		Doc:   catalog.Bundle{License: licence},
		Files: map[string][]byte{},
	}
}

// TestNoticeWording: who is credited and how, by publisher and licence.
func TestNoticeWording(t *testing.T) {
	const (
		tail   = " Source: https://hr.harnsy.dev/e/%s/sample. Changes: rendered for a test. Provided as is, without warranty of any kind (see the licence)."
		ccBy   = " Licensed under CC BY 4.0 (https://creativecommons.org/licenses/by/4.0/)."
		cc0    = " Dedicated to the public domain under CC0 1.0 (https://creativecommons.org/publicdomain/zero/1.0/)."
		none   = " No licence is stated for this employee: do not copy or redistribute it."
		listed = "\nhr.harnsy.dev lists this work; it is not made or endorsed by harnsy.\n"
		ours   = "\nThe names and logos «harnsy» and «hr.harnsy.dev», and the picture of this employee (not included here), are not covered by this licence.\n"
		theirs = "\nThe names and logos «harnsy» and «hr.harnsy.dev» are not covered by this licence.\n"
	)
	at := func(s, publisher string) string { return strings.Replace(s, "%s", publisher, 1) }
	cases := []struct {
		name string
		e    *catalog.Employee
		want string
	}{
		{"own, CC BY: the copyright notice", worker("harnsy", "harnsy", "CC-BY-4.0"),
			"Sample 1.0.0 by harnsy (Pavel Buchnev), © 2026 Pavel Buchnev (harnsy)." + ccBy + at(tail, "harnsy") + "\n" + ours},
		{"own, CC0: no copyright notice beside a dedication", worker("harnsy", "harnsy", "CC0-1.0"),
			"Sample 1.0.0 by harnsy (Pavel Buchnev)." + cc0 + at(tail, "harnsy") + "\n" + ours},
		{"own, no licence: all rights reserved", worker("harnsy", "harnsy", ""),
			"Sample 1.0.0 by harnsy (Pavel Buchnev), © 2026 Pavel Buchnev (harnsy)." + none + at(tail, "harnsy") + "\n" + ours},
		{"another publisher, CC BY: its copyright notice, not ours", worker("octo", "Octo Press", "CC-BY-4.0"),
			"Sample 1.0.0 by Octo Press, © 2026 Octo Press, published on hr.harnsy.dev." + ccBy + at(tail, "octo") + "\n" + listed + theirs},
		{"another publisher, CC0", worker("octo", "Octo Press", "CC0-1.0"),
			"Sample 1.0.0 by Octo Press." + cc0 + at(tail, "octo") + "\n" + listed + theirs},
		{"another publisher without a display name", worker("octo", "", "CC-BY-4.0"),
			"Sample 1.0.0 by octo, © 2026 octo, published on hr.harnsy.dev." + ccBy + at(tail, "octo") + "\n" + listed + theirs},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := notice(c.e, "rendered for a test"); got != c.want {
				t.Errorf("notice:\n%s\nwant:\n%s", got, c.want)
			}
		})
	}
}

// TestNoticeNoteIsMarkedAsThePublishers: the feed's licence note is the publisher's text, not part of the licence line.
func TestNoticeNoteIsMarkedAsThePublishers(t *testing.T) {
	for publisher, want := range map[string]string{
		"harnsy": "\nNote from harnsy: Keep the names.\n",
		"octo":   "\nNote from the publisher: Keep the names.\n",
	} {
		e := worker(publisher, "", "CC-BY-4.0")
		e.Entry.LicenseNote = "Keep the names."
		if got := notice(e, "x"); !strings.Contains(got, want) {
			t.Errorf("%s: the notice lacks %q:\n%s", publisher, want, got)
		}
	}
}

// TestNoticePointsAtSkillLicences: the pointer goes with a LICENSE file in a skill, whatever the bundle's expression.
func TestNoticePointsAtSkillLicences(t *testing.T) {
	const pointer = "\nSome skills carry another licence for the parts adapted from other works or for their scripts (for example MIT): it is the `LICENSE` file in that skill's folder, and it applies to those parts (the bundle's licence: %s).\n"
	cases := []struct {
		name, licence string
		files         []string
		want          bool
	}{
		{"a skill with a LICENSE under a plain expression", "CC-BY-4.0", []string{"skills/a/SKILL.md", "skills/a/LICENSE"}, true},
		{"a skill with a LICENSE under AND", "CC-BY-4.0 AND MIT", []string{"skills/a/LICENSE"}, true},
		{"AND without a LICENSE file", "CC-BY-4.0 AND MIT", []string{"skills/a/SKILL.md"}, false},
		{"a LICENSE deeper in a skill is not the skill's own", "CC-BY-4.0", []string{"skills/a/references/LICENSE"}, false},
		{"no skills", "CC-BY-4.0", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := worker("harnsy", "", c.licence)
			for _, f := range c.files {
				e.Files[f] = []byte("x")
			}
			want := strings.Replace(pointer, "%s", c.licence, 1)
			if got := notice(e, "x"); strings.Contains(got, want) != c.want {
				t.Errorf("pointer present = %v, want %v:\n%s", !c.want, c.want, got)
			}
		})
	}
}

// TestCreditWithoutAYear: a bundle that names no date gets a notice without a year, not an empty one.
func TestCreditWithoutAYear(t *testing.T) {
	e := worker("harnsy", "", "CC-BY-4.0")
	e.Entry.Published = ""
	if got, want := credit(e, "x"), "by harnsy (Pavel Buchnev), © Pavel Buchnev (harnsy)."; !strings.Contains(got, want) {
		t.Errorf("credit %q lacks %q", got, want)
	}
}
