package catalog_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/harnesy/hr-mcp/internal/catalog"
)

// words is n different made-up words, each of the given length.
func words(n, length int) string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("q%0*d", length-1, i)
	}
	return strings.Join(out, " ")
}

// TestSearchRefusesALongRequest: what a person types is short. A request above the limits is refused, not cut, and the
// refusal does not repeat what was sent; the work of a search is at most MaxQueryTerms words per employee.
func TestSearchRefusesALongRequest(t *testing.T) {
	r := staff(t)
	refused := []struct {
		name string
		q    catalog.Query
		want string
	}{
		{"64 KB of different words", catalog.Query{Text: words(8000, 7)}, "query"},
		{"one byte over the query limit", catalog.Query{Text: strings.Repeat("a", catalog.MaxQueryBytes+1)}, "query"},
		{"one word more than the most", catalog.Query{Text: words(catalog.MaxQueryTerms+1, 4)}, "query"},
		{"a long department", catalog.Query{Department: strings.Repeat("d", catalog.MaxDepartmentBytes+1)}, "department"},
		{"a long cursor", catalog.Query{Cursor: "7:" + strings.Repeat("0", catalog.MaxCursorBytes)}, "cursor"},
	}
	for _, c := range refused {
		t.Run(c.name, func(t *testing.T) {
			_, err := r.Search(c.q)
			if err == nil || !strings.HasPrefix(err.Error(), c.want) {
				t.Fatalf("got %v, want a refusal that starts with %q", err, c.want)
			}
			if len(err.Error()) > 200 || strings.Contains(err.Error(), "q000") || strings.Contains(err.Error(), "aaaa") || strings.Contains(err.Error(), "dddd") {
				t.Fatalf("the refusal must be short and must not repeat the request: %.300q", err)
			}
		})
	}
	if catalog.MaxQueryBytes != 256 || catalog.MaxQueryTerms != 16 {
		t.Fatalf("limits %d bytes, %d words; want 256 and 16", catalog.MaxQueryBytes, catalog.MaxQueryTerms)
	}
	// The largest request that is taken: MaxQueryTerms different words in MaxQueryBytes bytes.
	most := words(catalog.MaxQueryTerms, 15) + " "
	if len(most) != catalog.MaxQueryBytes {
		t.Fatalf("the test's largest query is %d bytes", len(most))
	}
	if _, err := r.Search(catalog.Query{Text: most, Department: strings.Repeat("d", catalog.MaxDepartmentBytes)}); err != nil {
		t.Fatalf("the largest request must be taken: %v", err)
	}
	// A word said again is one word: it neither counts against the limit nor adds to the score.
	again := strings.TrimSpace(strings.Repeat("invoices ", 28))
	p, err := r.Search(catalog.Query{Text: again})
	if err != nil || ids(p) != "bookkeeper accountant" {
		t.Fatalf("a repeated word: %q, %v", ids(p), err)
	}
}
