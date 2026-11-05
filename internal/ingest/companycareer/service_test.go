package companycareer

import (
	"context"
	"github.com/strelov1/freehire/internal/ingest/sources"
	"testing"
)

type fixtureSource struct{ jobs []sources.Job }

func (f fixtureSource) Provider() string { return "careerpage" }
func (f fixtureSource) Fetch(context.Context, sources.CompanyEntry) ([]sources.Job, error) {
	return f.jobs, nil
}
func TestPreviewDoesNotEnableUnreadableOrIncompleteJobs(t *testing.T) {
	for _, job := range []sources.Job{{Unreadable: true}, {Title: "Engineer", URL: "https://example.test/job/1"}} {
		s := &Service{Registry: map[string]sources.Source{"careerpage": fixtureSource{jobs: []sources.Job{job}}}}
		if _, err := s.Preview(context.Background(), Input{Company: "Example", URL: "https://example.test/careers"}); err == nil {
			t.Fatal("unverified collection was accepted")
		}
	}
}
func TestShortlistIsTenUniqueVerifiedMethodsOrExplicitFallbacks(t *testing.T) {
	seen := map[string]bool{}
	supported, fallback := 0, 0
	for _, seed := range Shortlist() {
		canonical, err := sources.CareerPageURL(seed.URL)
		if err != nil || canonical != seed.URL || seen[seed.URL] {
			t.Fatalf("invalid or duplicate source %s", seed.URL)
		}
		seen[seed.URL] = true
		if seed.Fallback != "" {
			fallback++
			continue
		}
		supported++
		e := sources.CompanyEntry{Company: seed.Company, Provider: seed.Provider, Board: seed.Board, CareerPage: seed.Config}
		if err := (sources.Config{Provider: e.Provider, Sources: []sources.CompanyEntry{e}}).Validate(sources.Taxonomy()); err != nil {
			t.Fatal(err)
		}
	}
	if supported != 6 || fallback != 4 {
		t.Fatalf("coverage count drifted: %d supported %d fallback", supported, fallback)
	}
}
