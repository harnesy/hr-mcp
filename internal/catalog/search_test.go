package catalog_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/harnesy/hr-mcp/internal/catalog"
	"github.com/harnesy/hr-mcp/internal/catalog/catalogtest"
)

// person is a bundle with the fields a search reads.
func person(name, department, tagline, competency string, keywords ...string) map[string][]byte {
	doc := map[string]any{
		"schema":  "harnsy.employee/v1",
		"persona": map[string]any{"name": "Ada"},
		"role": map[string]any{"name": name, "department": department, "tagline": tagline, "keywords": keywords,
			"prompt": "You are an AI agent."},
		"competencies": []any{map[string]any{"key": "c", "name": competency}},
		"license":      "CC-BY-4.0",
	}
	bj, _ := json.Marshal(doc)
	return map[string][]byte{"bundle.json": bj}
}

func staff(t *testing.T) *catalog.Release {
	t.Helper()
	k, dir := catalogtest.New(t), t.TempDir()
	k.Release(t, dir, []catalogtest.Packed{
		k.Pack(t, "harnsy/bookkeeper", "1.0.0", person("bookkeeper", "finance", "Keeps the ledger tidy every day.", "Ledger entries", "accounting records", "invoices"), nil),
		k.Pack(t, "harnsy/accountant", "1.0.0", person("accountant", "finance", "Closes your month on a calendar.", "Month-end close", "reconcile invoices"), nil),
		k.Pack(t, "harnsy/reviewer", "1.0.0", person("reviewer", "engineering", "Finds the race before it ships.", "Go review", "code review", "pull request"), nil),
		k.Pack(t, "harnsy/developer-go", "1.0.0", person("developer-go", "engineering", "Ships small Go services.", "Go services", "backend"), nil),
	}, nil)
	r, err := catalog.Load(dir, k.Key, catalogtest.Host, k.Now)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func ids(p catalog.Page) string {
	var out []string
	for _, e := range p.Employees {
		out = append(out, strings.TrimPrefix(e.Entry.ID, "harnsy/"))
	}
	return strings.Join(out, " ")
}

func TestSearch(t *testing.T) {
	r := staff(t)
	cases := []struct {
		name string
		q    catalog.Query
		want string
	}{
		{"no words: everyone, in the feed's order", catalog.Query{}, "bookkeeper accountant reviewer developer-go"},
		{"the title beats a keyword", catalog.Query{Text: "accountant"}, "accountant bookkeeper"},
		{"a phrase a person types", catalog.Query{Text: "Find me someone to review Go code"}, "reviewer developer-go"},
		{"more words matched come first", catalog.Query{Text: "go services backend"}, "developer-go reviewer"},
		{"a department alone", catalog.Query{Department: "finance"}, "bookkeeper accountant"},
		{"words within a department", catalog.Query{Text: "invoices", Department: "engineering"}, ""},
		{"any letter case", catalog.Query{Text: "INVOICES", Department: "Finance"}, "bookkeeper accountant"},
		{"nothing matches", catalog.Query{Text: "zeppelin pilot"}, ""},
		{"the persona's name", catalog.Query{Text: "ada", Limit: 2}, "bookkeeper accountant"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := r.Search(c.q)
			if err != nil {
				t.Fatal(err)
			}
			if got := ids(p); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestSearchPages(t *testing.T) {
	r := staff(t)
	var seen []string
	q := catalog.Query{Limit: 3}
	for page := 0; ; page++ {
		p, err := r.Search(q)
		if err != nil {
			t.Fatal(err)
		}
		seen = append(seen, ids(p))
		if p.Total != 4 {
			t.Fatalf("total %d, want 4", p.Total)
		}
		if p.Next == "" {
			break
		}
		if page > 3 {
			t.Fatal("paging does not end")
		}
		q.Cursor = p.Next
	}
	if got := strings.Join(seen, " | "); got != "bookkeeper accountant reviewer | developer-go" {
		t.Fatalf("pages %q", got)
	}
}

func TestSearchLimits(t *testing.T) {
	r := staff(t)
	for _, c := range []struct{ limit, want int }{{0, 4}, {-5, 4}, {1, 1}, {500, 4}} {
		p, err := r.Search(catalog.Query{Limit: c.limit})
		if err != nil || len(p.Employees) != c.want {
			t.Fatalf("limit %d: %d employees, err %v; want %d", c.limit, len(p.Employees), err, c.want)
		}
	}
	if catalog.MaxSearchLimit != 20 {
		t.Fatalf("the most a page holds is %d, want 20", catalog.MaxSearchLimit)
	}
	for _, cursor := range []string{"x", "-1", "7:99", fmt.Sprintf("%d:2", r.Feed.Seq+1)} {
		if _, err := r.Search(catalog.Query{Cursor: cursor}); err == nil {
			t.Fatalf("cursor %q must be refused", cursor)
		}
	}
}

// TestSearchPrefersTheExactWord: «accountant» and «account executive» share a stem; the one the person named comes first.
func TestSearchPrefersTheExactWord(t *testing.T) {
	k, dir := catalogtest.New(t), t.TempDir()
	k.Release(t, dir, []catalogtest.Packed{
		k.Pack(t, "harnsy/account-executive", "1.0.0", person("account executive", "sales", "Prepares your sales calls.", "Call preparation"), nil),
		k.Pack(t, "harnsy/accountant", "1.0.0", person("accountant", "finance", "Closes your month on a calendar.", "Month-end close"), nil),
	}, nil)
	r, err := catalog.Load(dir, k.Key, catalogtest.Host, k.Now)
	if err != nil {
		t.Fatal(err)
	}
	for text, want := range map[string]string{
		"accountant":             "accountant account-executive",
		"find an Accountants":    "accountant account-executive",
		"account executive":      "account-executive accountant",
		"accounting":             "account-executive accountant", // neither is named: the feed's order
		"an executive for sales": "account-executive",
	} {
		p, err := r.Search(catalog.Query{Text: text})
		if err != nil {
			t.Fatal(err)
		}
		if got := ids(p); got != want {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
}
