package catalog

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Page sizes of a search.
const (
	DefaultSearchLimit = 10
	MaxSearchLimit     = 20
)

// Query is a search of the catalog. Every field is optional: no words lists everyone in the feed's order.
type Query struct {
	Text       string // words a person types: a role, a skill, a need
	Department string // one of the catalog's departments, any letter case
	Limit      int    // at most MaxSearchLimit; 0 means DefaultSearchLimit
	Cursor     string // Page.Next of the page before
}

// Page is one page of a search's result.
type Page struct {
	Employees []*Employee
	Total     int    // how many employees match in all
	Next      string // the cursor of the next page; empty on the last
}

// Words too common in a request to tell one employee from another.
var stopWords = map[string]bool{"a": true, "an": true, "the": true, "and": true, "or": true, "for": true, "to": true,
	"of": true, "on": true, "in": true, "with": true, "who": true, "that": true, "can": true, "me": true, "my": true,
	"i": true, "we": true, "our": true, "need": true, "find": true, "want": true, "someone": true, "somebody": true,
	"employee": true, "agent": true, "hire": true, "install": true, "hr": true}

// A field's weight: where a word is found says how much the employee is about it.
const (
	weightName    = 5 // title, the id's name
	weightSkill   = 3 // competency names, tags, keywords
	weightSummary = 2
	weightOther   = 1 // department, industries, the persona's name
)

// searchIndex is an employee's searchable words by weight, made once when the release loads.
type searchIndex map[string]int

func indexOf(e Entry) searchIndex {
	ix := searchIndex{}
	add := func(weight int, texts ...string) {
		for _, t := range texts {
			for _, w := range words(t) {
				if ix[w] < weight {
					ix[w] = weight
				}
			}
		}
	}
	_, name, _ := strings.Cut(e.ID, "/")
	add(weightName, e.Title, name)
	add(weightSkill, e.Skills...)
	add(weightSkill, e.Tags...)
	add(weightSkill, e.Keywords...)
	add(weightSummary, e.Summary)
	add(weightOther, e.Department, e.Persona.Name)
	add(weightOther, e.Industries...)
	return ix
}

// words splits text into lowercase stems of letters and digits.
func words(text string) []string {
	parts := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	for i, p := range parts {
		parts[i] = stem(p)
	}
	return parts
}

// stem cuts a common English ending, so that «accountant» finds «accounting» and «reviews» finds «reviewer». It is a
// rough cut for matching, not grammar: a word keeps at least four letters.
func stem(w string) string {
	for _, end := range []string{"ments", "ment", "ings", "ing", "ants", "ant", "ers", "er", "ed", "es", "s"} {
		if cut, ok := strings.CutSuffix(w, end); ok && len(cut) >= 4 {
			return cut
		}
	}
	return w
}

// score is how well the index answers one word: the best weight among the words it matches.
func (ix searchIndex) score(word string) int {
	best := ix[word]
	if len(word) < 4 {
		return best // a short word («go», «ui») matches only itself
	}
	for w, weight := range ix {
		if weight > best && len(w) >= 4 && (strings.HasPrefix(w, word) || strings.HasPrefix(word, w)) {
			best = weight
		}
	}
	return best
}

// Search finds employees: those in the department, when one is given, that match at least one word of the text. The
// ones that match more words come first, then the better matches, then the feed's order.
func (r *Release) Search(q Query) (Page, error) {
	var terms []string
	for _, w := range words(q.Text) {
		if !stopWords[w] {
			terms = append(terms, w)
		}
	}
	type hit struct {
		emp            *Employee
		matched, score int
	}
	var hits []hit
	for _, e := range r.Employees {
		if q.Department != "" && !strings.EqualFold(q.Department, e.Entry.Department) {
			continue
		}
		h := hit{emp: e}
		for _, t := range terms {
			if s := e.index.score(t); s > 0 {
				h.matched++
				h.score += s
			}
		}
		if h.matched > 0 || len(terms) == 0 {
			hits = append(hits, h)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].matched != hits[j].matched {
			return hits[i].matched > hits[j].matched
		}
		return hits[i].score > hits[j].score
	})
	from, err := r.offset(q.Cursor, len(hits))
	if err != nil {
		return Page{}, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultSearchLimit
	}
	to := min(from+min(limit, MaxSearchLimit), len(hits))
	p := Page{Total: len(hits)}
	for _, h := range hits[from:to] {
		p.Employees = append(p.Employees, h.emp)
	}
	if to < len(hits) {
		p.Next = fmt.Sprintf("%d:%d", r.Feed.Seq, to)
	}
	return p, nil
}

// offset reads a cursor («<feed seq>:<offset>»): it is good only for the release that gave it.
func (r *Release) offset(cursor string, total int) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	seq, off, _ := strings.Cut(cursor, ":")
	s, err1 := strconv.ParseInt(seq, 10, 64)
	n, err2 := strconv.Atoi(off)
	if err1 != nil || err2 != nil || n < 0 || n >= total {
		return 0, errors.New("cursor: not a cursor of this search; search again without it")
	}
	if s != r.Feed.Seq {
		return 0, errors.New("cursor: the catalog changed since that page; search again without it")
	}
	return n, nil
}
