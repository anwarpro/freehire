package curriculum

import (
	"strings"
	"testing"
)

func engine(t *testing.T, minimum int) *Engine {
	t.Helper()
	e, err := New(Config{Tracks: []Track{{ID: "python", Label: "Python", Enabled: true, Minimum: minimum, Skills: []Skill{{Name: "Python", Aliases: []string{"python", "python3"}}, {Name: "Django", Aliases: []string{"django"}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func TestRequirementsPreferredAndLiteralBoundaries(t *testing.T) {
	e := engine(t, 1)
	result := e.Match("Backend engineer", "<h3>Requirements</h3><ul><li>Python is required for this role.</li></ul><h3>Preferred qualifications</h3><p>Django is a bonus.</p>")
	if len(result.Matches) != 1 {
		t.Fatalf("missing overlap: %+v", result)
	}
	ev := result.Matches[0].Evidence
	if len(ev) != 2 || ev[0].Kind != "required" || ev[1].Kind != "preferred" {
		t.Fatalf("lost qualifiers: %+v", ev)
	}
	if got := e.Match("Engineer", "We use CPythonic tools and djangonauts."); len(got.Matches) != 0 {
		t.Fatalf("substring false positive: %+v", got)
	}
	if got := e.Match("Engineer", "A Python implementation is discussed."); got.Matches[0].Evidence[0].Kind != "mentioned" {
		t.Fatal("unqualified mention became a requirement")
	}
}
func TestNegativeAndHiddenTextDoNotBecomeRequirements(t *testing.T) {
	for _, body := range []string{"Python is not required.", "Python (not required).", "Python, not required.", "Python is not mandatory.", "Python experience is not essential.", "Python skills are not required.", "We do not require Python.", "Python experience isn't needed.", "No Python experience required.", "No experience with Python is required.", "<script>Python required</script><p>Build services.</p>", "<style>.python { color: red; }</style>"} {
		if r := engine(t, 1).Match("Engineer", body); len(r.Matches) != 0 {
			t.Errorf("false evidence for %s: %+v", body, r)
		}
	}
}
func TestMinimumCountsDistinctSkillsNotRepeatedAliases(t *testing.T) {
	e := engine(t, 2)
	if len(e.Match("Python engineer", "Python and Python3 required.").Matches) != 0 {
		t.Fatal("one skill counted repeatedly")
	}
	if len(e.Match("Python engineer", "Django required.").Matches) != 1 {
		t.Fatal("two distinct skills lost")
	}
}
func TestEvidenceContainsTheMatchedTextEvenInLongParagraph(t *testing.T) {
	r := engine(t, 1).Match("Engineer", strings.Repeat("An unrelated sentence fragment ", 40)+"Python is required for building services.")
	if len(r.Matches) != 1 || !strings.Contains(r.Matches[0].Evidence[0].Text, "Python") {
		t.Fatal("evidence clipped away its matched alias")
	}
}
func TestDisabledTracksAndInvalidMappings(t *testing.T) {
	good := Config{Tracks: []Track{{ID: "python", Label: "Python", Enabled: false, Minimum: 1, Skills: []Skill{{Name: "Python", Aliases: []string{"Python"}}}}}}
	e, err := New(good)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Match("Python engineer", "").Matches) != 0 {
		t.Fatal("disabled track suggested")
	}
	good.Tracks[0].Skills[0].Aliases = append(good.Tracks[0].Skills[0].Aliases, "python")
	if _, err = New(good); err == nil {
		t.Fatal("duplicate alias accepted")
	}
	if _, err = New(Config{}); err == nil {
		t.Fatal("empty configuration accepted")
	}
}
func TestTitleEvidenceAndStrongestDescriptionEvidence(t *testing.T) {
	e := engine(t, 1)
	r := e.Match("Python engineer", "Python preferred. Python required.")
	if len(r.Matches) != 1 || len(r.Matches[0].Evidence) != 1 || r.Matches[0].Evidence[0].Kind != "required" {
		t.Fatalf("not strongest evidence: %+v", r)
	}
	if Hash("Engineer", "old") == Hash("Engineer", "new") {
		t.Fatal("edits not detected")
	}
}

func TestHeadingsWithoutColonsCarryTheirQualifier(t *testing.T) {
	r := engine(t, 1).Match("Engineer", "<h3>Requirements</h3><p>Python</p><h3>Nice to have</h3><p>Django</p>")
	if len(r.Matches) != 1 || r.Matches[0].Evidence[0].Kind != "required" || r.Matches[0].Evidence[1].Kind != "preferred" {
		t.Fatalf("lost section context: %+v", r)
	}
}
