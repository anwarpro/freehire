// Package curriculum suggests course relevance from explicit posting text.
// A suggestion is evidence of overlap, never a claim that a student qualifies.
package curriculum

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/strelov1/freehire/internal/platform/htmltext"
)

type Skill struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}
type Track struct {
	ID      string  `json:"id"`
	Label   string  `json:"label"`
	Enabled bool    `json:"enabled"`
	Minimum int     `json:"minimum"`
	Skills  []Skill `json:"skills"`
}
type Config struct {
	Tracks []Track `json:"tracks"`
}
type Evidence struct {
	Skill string `json:"skill"`
	Alias string `json:"alias"`
	Kind  string `json:"kind"`
	Text  string `json:"text"`
}
type Match struct {
	TrackID  string     `json:"track_id"`
	Evidence []Evidence `json:"evidence"`
}
type Result struct {
	Matches   []Match `json:"matches"`
	Truncated bool    `json:"truncated"`
}
type compiledSkill struct {
	skill    Skill
	patterns []*regexp.Regexp
}
type compiledTrack struct {
	track  Track
	skills []compiledSkill
}
type Engine struct{ tracks []compiledTrack }

var trackID = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,39}$`)

func clean(s string, max int) bool {
	return strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= max && !strings.ContainsFunc(s, unicode.IsControl)
}
func New(c Config) (*Engine, error) {
	if len(c.Tracks) == 0 || len(c.Tracks) > 12 {
		return nil, fmt.Errorf("Choose 1–12 curriculum tracks")
	}
	e := &Engine{}
	seen := map[string]bool{}
	for _, t := range c.Tracks {
		if !trackID.MatchString(t.ID) || seen[t.ID] || !clean(t.Label, 100) || len(t.Skills) == 0 || len(t.Skills) > 30 || t.Minimum < 1 || t.Minimum > len(t.Skills) {
			return nil, fmt.Errorf("Each track needs a unique stable ID, label, 1–30 skills and a valid minimum")
		}
		seen[t.ID] = true
		ct := compiledTrack{track: t}
		names := map[string]bool{}
		aliases := map[string]bool{}
		for _, s := range t.Skills {
			if !clean(s.Name, 80) || strings.Contains(s.Name, "|") || names[strings.ToLower(s.Name)] || len(s.Aliases) == 0 || len(s.Aliases) > 8 {
				return nil, fmt.Errorf("Each skill needs a unique name and 1–8 aliases")
			}
			names[strings.ToLower(s.Name)] = true
			cs := compiledSkill{skill: s}
			for _, a := range s.Aliases {
				a = strings.TrimSpace(a)
				key := strings.ToLower(a)
				if !clean(a, 64) || strings.ContainsAny(a, ",|") || aliases[key] {
					return nil, fmt.Errorf("Skill aliases must be nonempty, at most 64 characters and unique within a track")
				}
				aliases[key] = true
				cs.patterns = append(cs.patterns, regexp.MustCompile(`(?i)(^|[^\pL\pN_])(`+regexp.QuoteMeta(a)+`)($|[^\pL\pN_])`))
			}
			ct.skills = append(ct.skills, cs)
		}
		e.tracks = append(e.tracks, ct)
	}
	return e, nil
}
func Decode(raw []byte) (Config, error) { var c Config; err := json.Unmarshal(raw, &c); return c, err }
func Hash(title, description string) string {
	sum := sha256.Sum256([]byte("curriculum-v1\x00" + title + "\x00" + description))
	return hex.EncodeToString(sum[:])
}

var sentences = regexp.MustCompile(`[.!?]\s+|[\n;]+`)
var preferred = regexp.MustCompile(`(?i)\b(preferred|preferable|nice.to.have|good.to.have|bonus|advantage|optional|desirable|a plus)\b`)
var required = regexp.MustCompile(`(?i)\b(required|requirements|qualifications|must|essential|mandatory|proficien\w*|experience with|knowledge of)\b`)
var heading = regexp.MustCompile(`(?i)^(responsibilities|about (us|the company)|benefits|what we offer|our (team|culture)|description|duties)\b`)
var preferredSection = regexp.MustCompile(`(?i)^(preferred( qualifications| skills)?|nice.to.have|good.to.have|optional( skills)?|desirable( skills)?)[: ]*$`)
var requiredSection = regexp.MustCompile(`(?i)^(requirements|qualifications|essential skills|must.have|what you (need|bring))[: ]*$`)
var negativeBefore = regexp.MustCompile(`(?i)\b(no|without|not (using|requiring|require|need|expect))\s+(prior\s+)?((experience|knowledge|proficiency)\s+(in|with|of)\s+)?$`)
var negativeAfter = regexp.MustCompile(`(?i)^\s*[,(:-]?\s*((skills|experience|knowledge|proficiency)\s+)?((is|are)\s+)?(not (strictly )?(required|needed|necessary|mandatory|essential)|(isn['’]t|aren['’]t) (required|needed|necessary|mandatory|essential))\b`)

func cut(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
func priority(s string) int {
	switch s {
	case "required":
		return 4
	case "preferred":
		return 3
	case "title":
		return 2
	default:
		return 1
	}
}

// Match uses literal aliases with Unicode boundaries and keeps the strongest
// observed evidence per skill. Unlabelled text stays "mentioned", not "required".
func (e *Engine) Match(title, description string) Result {
	out := Result{Matches: []Match{}, Truncated: utf8.RuneCountInString(description) > 50000}
	type segment struct{ text, kind string }
	segments := []segment{{text: cut(title, 500), kind: "title"}}
	section := "mentioned"
	for _, line := range sentences.Split(htmltext.ToText(description), -1) {
		line = strings.TrimSpace(strings.TrimLeft(line, "-*• "))
		if line == "" {
			continue
		}
		if heading.MatchString(line) {
			section = "mentioned"
		}
		if preferredSection.MatchString(line) {
			section = "preferred"
		} else if requiredSection.MatchString(line) {
			section = "required"
		}
		kind := section
		if preferred.MatchString(line) {
			kind = "preferred"
		} else if required.MatchString(line) {
			kind = "required"
		}
		if utf8.RuneCountInString(line) < 100 && (strings.HasSuffix(line, ":") || strings.EqualFold(line, "Requirements") || strings.EqualFold(line, "Qualifications") || strings.EqualFold(line, "Preferred qualifications")) {
			section = kind
		}
		segments = append(segments, segment{text: line, kind: kind})
	}
	for _, ct := range e.tracks {
		if !ct.track.Enabled {
			continue
		}
		m := Match{TrackID: ct.track.ID, Evidence: []Evidence{}}
		for _, cs := range ct.skills {
			var best *Evidence
			for _, seg := range segments {
				for i, p := range cs.patterns {
					for _, at := range p.FindAllStringSubmatchIndex(seg.text, -1) {
						start, end := at[4], at[5]
						prefix := seg.text[:start]
						suffix := seg.text[end:]
						if negativeBefore.MatchString(prefix) || negativeAfter.MatchString(suffix) {
							continue
						}
						if best == nil || priority(seg.kind) > priority(best.Kind) {
							best = &Evidence{Skill: cs.skill.Name, Alias: cs.skill.Aliases[i], Kind: seg.kind, Text: evidenceText(seg.text, start, end)}
						}
					}
				}
			}
			if best != nil {
				m.Evidence = append(m.Evidence, *best)
			}
		}
		if len(m.Evidence) >= ct.track.Minimum {
			out.Matches = append(out.Matches, m)
		}
	}
	return out
}
func (r Result) TrackIDs() []string {
	out := []string{}
	for _, m := range r.Matches {
		out = append(out, m.TrackID)
	}
	slices.Sort(out)
	return out
}

func evidenceText(text string, start, end int) string {
	runes := []rune(text)
	at := utf8.RuneCountInString(text[:start])
	to := utf8.RuneCountInString(text[:end])
	left := max(0, at-150)
	right := min(len(runes), max(to+150, left+350))
	value := strings.Join(strings.Fields(string(runes[left:right])), " ")
	if left > 0 {
		value = "…" + value
	}
	if right < len(runes) {
		value += "…"
	}
	return value
}
