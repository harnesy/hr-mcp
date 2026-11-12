package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/harnesy/hr-mcp/internal/catalog"
)

// SearchIn is the input of search_employees.
type SearchIn struct {
	Query      string `json:"query,omitempty" jsonschema:"a few words for a role, a skill or a need, e.g. accountant, or: review Go code; at most 256 bytes and 16 different words, a longer one is refused"`
	Department string `json:"department,omitempty" jsonschema:"one of: engineering, product, design, marketing, sales, support, operations, finance, legal, management, content, data, hr"`
	Limit      int    `json:"limit,omitempty" jsonschema:"cards per page, at most 20; 10 when left out"`
	Cursor     string `json:"cursor,omitempty" jsonschema:"next_cursor of the page before"`
}

// SearchOut is the answer of search_employees.
type SearchOut struct {
	Employees  []Card `json:"employees"`
	Total      int    `json:"total" jsonschema:"how many employees match in all"`
	NextCursor string `json:"next_cursor,omitempty" jsonschema:"pass as cursor for the next page; absent on the last page"`
}

// Card is an employee as a search shows it.
type Card struct {
	ID         string      `json:"id" jsonschema:"publisher/name, e.g. harnsy/accountant"`
	Version    string      `json:"version"`
	Title      string      `json:"title"`
	Summary    string      `json:"summary"`
	Persona    PersonaCard `json:"persona"`
	Department string      `json:"department,omitempty"`
	Skills     []string    `json:"skills" jsonschema:"what it is good at, as the catalog's card names it"`
	License    string      `json:"license,omitempty" jsonschema:"the licence of its text, SPDX"`
	CVURL      string      `json:"cv_url,omitempty" jsonschema:"its page on hr.harnsy.dev"`
}

// PersonaCard is the employee's made-up name.
type PersonaCard struct {
	Name string `json:"name"`
}

func card(e *catalog.Employee) Card {
	c := Card{ID: e.Entry.ID, Version: e.Entry.Version, Title: e.Entry.Title, Summary: e.Entry.Summary,
		Persona: PersonaCard{Name: e.Entry.Persona.Name}, Department: e.Entry.Department, Skills: e.Entry.Skills,
		License: e.Entry.License, CVURL: e.Entry.CVURL}
	if c.Skills == nil {
		c.Skills = []string{}
	}
	return c
}

func (s *server) search(_ context.Context, _ *mcp.CallToolRequest, in SearchIn) (*mcp.CallToolResult, SearchOut, error) {
	out := SearchOut{Employees: []Card{}}
	rel, err := s.release()
	if err != nil {
		return nil, out, s.logged("search_employees", "", err)
	}
	p, err := rel.Search(catalog.Query{Text: in.Query, Department: in.Department, Limit: in.Limit, Cursor: in.Cursor})
	if err != nil {
		return nil, out, s.logged("search_employees", "", err)
	}
	for _, e := range p.Employees {
		out.Employees = append(out.Employees, card(e))
	}
	out.Total, out.NextCursor = p.Total, p.Next
	return nil, out, s.logged("search_employees", "", nil)
}
