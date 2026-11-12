package catalog

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// The names of a release, in the grammar of the catalog's publisher: the expressions and the file-name rule below are
// the publisher's own, neither looser nor stricter. The feed is signed, and signed is not checked: an id becomes a
// folder on a person's disk, a line of YAML and a word of a command there, so a release that names anything outside
// the grammar is refused whole.
var (
	partRE      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`) // a publisher, an employee's name
	semverRE    = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	avatarRE    = regexp.MustCompile(`^avatar\.(png|jpe?g|webp)$`)
	skillNameRE = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)
	skillFileRE = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`) // a file of a skill's folder
)

// CasebookFile is the path of the casebook in a bundle.
const CasebookFile = "docs/persona-casebook.md"

// The files a skill carries beside its folders, and the folders: flat, of plain files.
const (
	skillDoc     = "SKILL.md"
	skillLicense = "LICENSE"
)

var skillFolders = []string{"scripts", "references", "assets"}

// windowsReserved are device names Windows opens instead of a file, with any extension.
var windowsReserved = map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true}

// ValidID reports whether id is <publisher>/<name> in the catalog's grammar.
func ValidID(id string) bool {
	publisher, name, ok := strings.Cut(id, "/")
	return ok && partRE.MatchString(publisher) && partRE.MatchString(name)
}

// checkNames refuses a feed entry whose id or version is outside the grammar.
func checkNames(e Entry) error {
	if !ValidID(e.ID) {
		return fmt.Errorf("id %.130q: want <publisher>/<name> in lowercase latin letters, digits and -", e.ID)
	}
	if !semverRE.MatchString(e.Version) {
		return fmt.Errorf("%s: version %.40q: want MAJOR.MINOR.PATCH", e.ID, e.Version)
	}
	return nil
}

// bundlePath reports whether a manifest may list the path: bundle.json, the casebook, the picture, and under
// skills/<skill>/ the files SKILL.md and LICENSE and the plain files of scripts/, references/ and assets/.
func bundlePath(p string) bool {
	if p == "bundle.json" || p == CasebookFile || avatarRE.MatchString(p) {
		return true
	}
	parts := strings.Split(p, "/")
	if len(parts) < 3 || parts[0] != "skills" || !skillNameRE.MatchString(parts[1]) {
		return false
	}
	switch len(parts) {
	case 3:
		return parts[2] == skillDoc || parts[2] == skillLicense
	case 4:
		return slices.Contains(skillFolders, parts[2]) && skillFileName(parts[3])
	}
	return false
}

// skillFileName reports whether name is a file name a skill's folder may hold: a portable name no file system reads
// as another, not hidden, not ending in a dot, not a device name on Windows, not SKILL.md in any letter case.
func skillFileName(name string) bool {
	if !skillFileRE.MatchString(name) || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.EqualFold(name, skillDoc) {
		return false
	}
	stem, _, _ := strings.Cut(name, ".")
	return !windowsReserved[strings.ToUpper(stem)]
}
