package handler

import (
	"github.com/google/uuid"
	"github.com/strelov1/freehire/internal/ai/assistant"
	"github.com/strelov1/freehire/internal/job/curriculum"
	"strings"
	"testing"
)

func TestTeamJobBotEnglishReadOnlyPrompt(t *testing.T) {
	h := &assistantHandlers{}
	names := h.registry(assistant.Session{Preset: assistant.PresetTeamJobs}, uuid.New()).Names()
	if len(names) != 5 {
		t.Fatal(names)
	}
	prompt := assistant.SystemPrompt(assistant.PresetTeamJobs, "ru")
	for _, text := range []string{"Reply to the candidate in English", "untrusted source material", "automatic", "original source URL", "student is eligible", "closed"} {
		if !strings.Contains(prompt, text) {
			t.Fatalf("missing prompt constraint %q", text)
		}
	}
	for _, name := range []string{"get_profile", "experience_add", "track_job", "inbox_list", "cv_edit"} {
		if strings.Contains(prompt, "`"+name+"`") {
			t.Fatal("candidate tool mentioned")
		}
	}
}

func TestTeamBotEvidenceBound(t *testing.T) {
	engine, err := curriculum.New(curriculum.Config{Tracks: []curriculum.Track{{ID: "python", Label: "Python", Enabled: true, Minimum: 1, Skills: []curriculum.Skill{{Name: "Python", Aliases: []string{"python"}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	result := teamBotEvidence(engine, "Office role", strings.Repeat("x", 50001)+" Python required")
	if !result.Truncated || len(result.Matches) != 0 {
		t.Fatal("long source was not bounded")
	}
}
