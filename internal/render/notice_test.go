package render_test

import (
	"strings"
	"testing"

	"github.com/harnesy/hr-mcp/internal/render"
)

// TestNotice: the credit that travels with an employee says what CC BY 4.0 asks for.
func TestNotice(t *testing.T) {
	r := staff(t)
	cases := []struct {
		id   string
		want []string
		not  []string
	}{
		{"harnsy/sample-plain", []string{
			"Sample librarian 1.0.0 by harnsy (Pavel Buchnev), © 2026 Pavel Buchnev (harnsy).",
			"Licensed under CC BY 4.0 (https://creativecommons.org/licenses/by/4.0/).",
			"Source: https://hr.harnsy.dev/e/harnsy/sample-plain.",
			"Changes: rendered for generic.",
			"Provided as is, without warranty of any kind (see the licence).",
		}, []string{"Built on", "LICENSE"}},
		{"harnsy/sample-skills", []string{
			"Built on:",
			"- A Made-up Guide to Shelves — Example Author, https://example.org/shelves, CC-BY-4.0, modified",
			"- Loan Desk Notes, https://example.org/loans, CC0-1.0, not modified",
		}, []string{"LICENSE"}},
		{"harnsy/sample-scripts", []string{
			"Licensed under CC BY 4.0 (https://creativecommons.org/licenses/by/4.0/).",
			"it is the `LICENSE` file in that skill's folder, and it applies to those parts (the bundle's licence: CC-BY-4.0 AND MIT).",
			"Note from harnsy: No rights in the names of the people mentioned are granted or implied.",
			"and the picture of this employee (not included here), are not covered by this licence.",
			"- Label Sizes Table, https://example.org/labels, MIT, modified",
		}, nil},
		{"octo/sample-guest", []string{
			"Sample guest 1.0.0 by Octo Press. Dedicated to the public domain under CC0 1.0 (https://creativecommons.org/publicdomain/zero/1.0/).",
			"hr.harnsy.dev lists this work; it is not made or endorsed by harnsy.",
			"The names and logos «harnsy» and «hr.harnsy.dev» are not covered by this licence.",
			"Source: https://hr.harnsy.dev/e/octo/sample-guest.",
		}, []string{"Pavel Buchnev", "CC BY 4.0", "©", "picture"}},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			tree := mustRender(t, r.Employee(c.id), render.Generic)
			for _, w := range c.want {
				if !strings.Contains(tree.Notice, w) {
					t.Errorf("the notice lacks %q:\n%s", w, tree.Notice)
				}
			}
			for _, n := range c.not {
				if strings.Contains(tree.Notice, n) {
					t.Errorf("the notice must not say %q:\n%s", n, tree.Notice)
				}
			}
		})
	}
}
