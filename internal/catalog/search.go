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

// Limits of a request. What a person types is short; a longer request is refused, not cut: a search costs one pass
// over every employee's words per word asked, so the words asked are few.
const (
	MaxQueryBytes      = 256 // Query.Text
	MaxQueryTerms      = 16  // different words of it that are searched for
	MaxDepartmentBytes = 64  // Query.Department
	MaxCursorBytes     = 64  // Query.Cursor
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

// searchIndex is an employee's searchable words, made once when the release loads.
type searchIndex struct {
	weights map[string]int  // stems, each with the weight of the best field it stands in
	named   map[string]bool // the words of its title and id as written, in the singular
}

func indexOf(e Entry) searchIndex {
	ix := searchIndex{weights: map[string]int{}, named: map[string]bool{}}
	add := func(weight int, texts ...string) {
		for _, t := range texts {
			for _, w := range tokens(t) {
				if w = stem(w); ix.weights[w] < weight {
					ix.weights[w] = weight
				}
			}
		}
	}
	_, name, _ := strings.Cut(e.ID, "/")
	add(weightName, e.Title, name)
	for _, w := range tokens(e.Title + " " + name) {
		ix.named[singular(w)] = true
	}
	add(weightSkill, e.Skills...)
	add(weightSkill, e.Tags...)
	add(weightSkill, e.Keywords...)
	add(weightSummary, e.Summary)
	add(weightOther, e.Department, e.Persona.Name)
	add(weightOther, e.Industries...)
	return ix
}

// tokens splits text into lowercase words of letters and digits.
func tokens(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// singular drops a plural s, so that «accountants» names the «accountant».
func singular(w string) string {
	if cut, ok := strings.CutSuffix(w, "s"); ok && len(cut) >= 4 {
		return cut
	}
	return w
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

// term is one word of a request: as written, in the singular, and its stem.
type term struct{ word, stem string }

// terms reads a request's words, each once, without the ones that tell nothing.
func terms(text string) []term {
	var out []term
	seen := map[term]bool{}
	for _, w := range tokens(text) {
		if t := (term{singular(w), stem(w)}); !stopWords[t.stem] && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// asked checks a request against the limits and reads its words. A refusal never repeats what was sent.
func (q Query) asked() ([]term, error) {
	switch {
	case len(q.Text) > MaxQueryBytes:
		return nil, fmt.Errorf("query: %d bytes, at most %d; say it in a few words", len(q.Text), MaxQueryBytes)
	case len(q.Department) > MaxDepartmentBytes:
		return nil, fmt.Errorf("department: at most %d bytes; it is one word, for example finance", MaxDepartmentBytes)
	case len(q.Cursor) > MaxCursorBytes:
		return nil, errors.New("cursor: not a cursor of this search; search again without it")
	}
	words := terms(q.Text)
	if len(words) > MaxQueryTerms {
		return nil, fmt.Errorf("query: %d different words, at most %d; say it in a few words", len(words), MaxQueryTerms)
	}
	return words, nil
}

// hit is how well one employee answers a request: how many of its words match, how well, and how many of them the
// employee is named by.
type hit struct {
	emp                   *Employee
	matched, score, named int
}

func (ix searchIndex) hit(e *Employee, terms []term) hit {
	h := hit{emp: e}
	for _, t := range terms {
		if s := ix.score(t.stem); s > 0 {
			h.matched++
			h.score += s
		}
		if ix.named[t.word] {
			h.named++
		}
	}
	return h
}

// score is how well the index answers one word: the best weight among the words it matches.
func (ix searchIndex) score(word string) int {
	best := ix.weights[word]
	if len(word) < 4 {
		return best // a short word («go», «ui») matches only itself
	}
	for w, weight := range ix.weights {
		if weight > best && len(w) >= 4 && (strings.HasPrefix(w, word) || strings.HasPrefix(word, w)) {
			best = weight
		}
	}
	return best
}

// Search finds employees: those in the department, when one is given, that match at least one word of the text. A
// request above the limits (MaxQueryBytes, MaxQueryTerms, MaxDepartmentBytes, MaxCursorBytes) is refused. The
// ones that match more words come first, then the better matches, then the ones named by the very word («accountant»
// before «account executive»), then the feed's order.
func (r *Release) Search(q Query) (Page, error) {
	asked, err := q.asked()
	if err != nil {
		return Page{}, err
	}
	var hits []hit
	for _, e := range r.Employees {
		if q.Department != "" && !strings.EqualFold(q.Department, e.Entry.Department) {
			continue
		}
		if h := e.index.hit(e, asked); h.matched > 0 || len(asked) == 0 {
			hits = append(hits, h)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.matched != b.matched {
			return a.matched > b.matched
		}
		if a.score != b.score {
			return a.score > b.score
		}
		return a.named > b.named
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
