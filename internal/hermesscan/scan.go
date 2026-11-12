// Package hermesscan tells whether a text would be blocked by Hermes Agent when it is a profile's SOUL.md.
//
// Hermes scans a SOUL.md that a profile distribution installed each time it builds the prompt, and on any hit
// leaves the whole file out. The patterns below are the ones of its scanner for context files (Hermes Agent v0.23.1,
// tools/threat_patterns.py, the entries of scope "all" and "context"; github.com/NousResearch/hermes-agent,
// © 2025 Nous Research, MIT licence: its text is the LICENSE file in this folder). They are here so that a rendered employee can be checked before it reaches a person's Hermes; the
// scanner in Hermes itself stays the judge.
//
// Three differences from the original, each on the strict side or without effect on ordinary text: a repeat of
// {0,2048} is written * (Go's limit is 1000); \w is written out with letters and digits of every script, and every
// kind of space is read as a plain space, as Python does; the text is not brought to the NFKC form first, so a word
// spelled with full-width or other look-alike letters is not caught here.
package hermesscan

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var patterns = compile([][2]string{
	{"prompt_injection", `ignore\s+(?:[\pL\pN_]+\s+){0,8}(previous|all|above|prior)\s+(?:[\pL\pN_]+\s+){0,8}instructions`},
	{"sys_prompt_override", `system\s+prompt\s+override`},
	{"disregard_rules", `disregard\s+(?:[\pL\pN_]+\s+){0,8}(your|all|any)\s+(?:[\pL\pN_]+\s+){0,8}(instructions|rules|guidelines)`},
	{"bypass_restrictions", `act\s+as\s+(if|though)\s+(?:[\pL\pN_]+\s+){0,8}you\s+(?:[\pL\pN_]+\s+){0,8}(have\s+no|don\'t\s+have)\s+(?:[\pL\pN_]+\s+){0,8}(restrictions|limits|rules)`},
	{"html_comment_injection", `<!--[^>]{0,512}(?:ignore|override|system|secret|hidden)[^>]{0,512}-->`},
	{"hidden_div", `<\s*div\s+style\s*=\s*["\'][^>]*display\s*:\s*none`},
	{"translate_execute", `translate\s+[^\n]{0,512}\s+into\s+[\pL\pN_]+(?:[\s-]+[\pL\pN_]+){0,2}\s+and\s+(execute|run|eval)\b`},
	{"deception_hide", `do\s+not\s+(?:[\pL\pN_]+\s+){0,8}tell\s+(?:[\pL\pN_]+\s+){0,8}the\s+user`},
	{"exfil_curl", `curl\s+[^\n]*\$\{?[\pL\pN_]*(?:KEY|TOKEN|SECRET|PASSWORD|CREDENTIAL)S?\b`},
	{"exfil_wget", `wget\s+[^\n]*\$\{?[\pL\pN_]*(?:KEY|TOKEN|SECRET|PASSWORD|CREDENTIAL)S?\b`},
	{"read_secrets", `cat\s+[^\n]*(\.env|credentials|\.netrc|\.pgpass|\.npmrc|\.pypirc)`},
	{"role_hijack", `you\s+are\s+(?:[\pL\pN_]+\s+){0,8}now\s+(?:a|an|the)\s+`},
	{"role_pretend", `pretend\s+(?:[\pL\pN_]+\s+){0,8}(you\s+are|to\s+be)\s+`},
	{"leak_system_prompt", `output\s+(?:[\pL\pN_]+\s+){0,8}(system|initial)\s+prompt`},
	{"remove_filters", `(respond|answer|reply)\s+without\s+(?:[\pL\pN_]+\s+){0,8}(restrictions|limitations|filters|safety)`},
	{"fake_update", `you\s+have\s+been\s+(?:[\pL\pN_]+\s+){0,8}(updated|upgraded|patched)\s+to`},
	{"identity_override", `\bname\s+yourself\s+[\pL\pN_]+`},
	{"c2_node_registration", `register\s+(as\s+)?a?\s*node`},
	{"c2_heartbeat", `(heartbeat|beacon|check[\s\-]?in)\s+(to|with)\s+`},
	{"c2_task_pull", `pull\s+(down\s+)?(?:new\s+)?task(?:ing|s)?\b`},
	{"c2_network_connect", `connect\s+to\s+the\s+network\b`},
	{"forced_action", `you\s+must\s+(?:[\pL\pN_]+\s+){0,3}(register|connect|report|beacon)\b`},
	{"anti_forensic_oneliner", `only\s+use\s+one[\s\-]?liners?\b`},
	{"anti_forensic_disk", `never\s+(?:[\pL\pN_]+\s+){0,8}(?:create|write)\s+(?:[\pL\pN_]+\s+){0,8}(?:script|file)\s+(?:[\pL\pN_]+\s+){0,8}disk`},
	{"env_var_unset_agent", `unset\s+[\pL\pN_]*(?:CLAUDE|CODEX|HERMES|AGENT|OPENAI|ANTHROPIC)[\pL\pN_]*`},
	{"known_c2_framework", `\b(?:cobalt\s*strike|sliver|havoc|mythic|metasploit|brainworm)\b`},
	{"c2_explicit", `\bc2\s+(?:server|channel|infrastructure|beacon)\b`},
	{"c2_explicit_long", `\bcommand\s+and\s+control\b`},
})

// invisible are characters a reader does not see; one of them anywhere is a hit. An emoji made of joined signs has one.
var invisible = []rune{0x200B, 0x200C, 0x200D, 0x2060, 0x2062, 0x2063, 0x2064, 0xFEFF, 0x202A, 0x202B, 0x202C, 0x202D, 0x202E, 0x2066, 0x2067, 0x2068, 0x2069}

type pattern struct {
	id string
	re *regexp.Regexp
}

func compile(rows [][2]string) []pattern {
	out := make([]pattern, len(rows))
	for i, r := range rows {
		out[i] = pattern{r[0], regexp.MustCompile("(?i)" + r[1])}
	}
	return out
}

// Patterns is the number of patterns.
func Patterns() int { return len(patterns) }

// Scan returns the ids of the patterns the text trips, in the scanner's order; none means the text would load.
func Scan(text string) []string {
	text = strings.TrimPrefix(text, "\uFEFF") // a leading byte order mark is dropped before the scan
	var hits []string
	plain := strings.Map(func(r rune) rune {
		if r != '\n' && unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, text)
	for _, p := range patterns {
		if p.re.MatchString(plain) {
			hits = append(hits, p.id)
		}
	}
	for _, r := range invisible {
		if strings.ContainsRune(text, r) {
			hits = append(hits, fmt.Sprintf("invisible_unicode_U+%04X", r))
		}
	}
	return hits
}
