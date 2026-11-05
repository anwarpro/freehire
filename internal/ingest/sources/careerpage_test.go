package sources

import (
	"context"
	"fmt"
	"golang.org/x/net/html"
	"strings"
	"testing"
)

type careerFixture map[string]string

func (f careerFixture) GetHTML(_ context.Context, u string) (*html.Node, error) {
	s, ok := f[u]
	if !ok {
		return nil, fmt.Errorf("unreadable fixture")
	}
	return html.Parse(strings.NewReader(s))
}
func TestCompanyCareerPagesPaginationAndIdentity(t *testing.T) {
	fixture := careerFixture{
		"https://example.test/careers":        `<a class="job" href="/job/a">A</a><a rel="next" href="/careers?page=2">Next</a>`,
		"https://example.test/careers?page=2": `<a class="job" href="/job/a">A</a><a class="job" href="/job/b">B</a>`,
		"https://example.test/job/a":          `<h1>Python Engineer</h1><article>Build Python APIs<script>alert(1)</script></article>`,
		"https://example.test/job/b":          `<h1>React Engineer</h1><article>Build interfaces</article>`,
	}
	e := CompanyEntry{Company: "Example BD", Provider: "careerpage", Board: "https://example.test/careers", CareerPage: &CareerPageConfig{Links: "a.job", Body: "article", Next: `a[rel="next"]`, DefaultLocation: "Dhaka, Bangladesh"}}
	first, err := NewCompanyCareerPage(fixture).Fetch(context.Background(), e)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || first[0].ExternalID == first[1].ExternalID || first[0].Title != "Python Engineer" || strings.Contains(first[0].Description, "script") {
		t.Fatalf("bad jobs: %+v", first)
	}
	fixture["https://example.test/job/a"] = `<h1>Updated Python Engineer</h1><article>Updated requirements</article>`
	second, err := NewCompanyCareerPage(fixture).Fetch(context.Background(), e)
	if err != nil || second[0].ExternalID != first[0].ExternalID || second[0].Title == first[0].Title {
		t.Fatalf("identity changed on update: %+v %v", second, err)
	}
	fixture["https://example.test/careers?page=2"] = `<title>Please wait</title>`
	if _, err := NewCompanyCareerPage(fixture).Fetch(context.Background(), e); err == nil {
		t.Fatal("accepted a challenge on a later page")
	}
}
func TestCompanyCareerPagesEmptyOrChangedMarkup(t *testing.T) {
	e := CompanyEntry{Company: "Empty", Provider: "careerpage", Board: "https://example.test/careers", CareerPage: &CareerPageConfig{Links: "a.job", Body: "article", EmptyText: "No current openings"}}
	f := careerFixture{e.Board: `<main>No current openings</main>`}
	jobs, err := NewCompanyCareerPage(f).Fetch(context.Background(), e)
	if err != nil || len(jobs) != 0 {
		t.Fatalf("explicit empty listing: %v", err)
	}
	f[e.Board] = `<main>Checking your browser</main>`
	if _, err := NewCompanyCareerPage(f).Fetch(context.Background(), e); err == nil {
		t.Fatal("challenge counted as empty")
	}
	f[e.Board] = `<a class="job" href="https://other.test/job/1">Job</a>`
	if _, err := NewCompanyCareerPage(f).Fetch(context.Background(), e); err == nil {
		t.Fatal("unexpected detail host accepted")
	}
}
func TestCompanyCareerPagesStructuredJobsAndValidation(t *testing.T) {
	e := CompanyEntry{Company: "Example", Provider: "careerpage", Board: "https://example.test/careers"}
	f := careerFixture{e.Board: `<script type="application/ld+json">{"@graph":[{"@type":"JobPosting","title":"Go Engineer","url":"https://example.test/job/1","description":"Build APIs","datePosted":"2026-01-01","validThrough":"2020-01-01"}]}</script>`}
	jobs, err := NewCompanyCareerPage(f).Fetch(context.Background(), e)
	if err != nil || len(jobs) != 1 || !jobs[0].Removed {
		t.Fatalf("invalid structured jobs: %+v %v", jobs, err)
	}
	for _, raw := range []string{"http://example.test/jobs", "https://user:secret@example.test/jobs", "https://example.test:8080/jobs", "javascript:alert(1)"} {
		if _, err := CareerPageURL(raw); err == nil {
			t.Fatalf("accepted invalid URL %s", raw)
		}
	}
	if err := (CareerPageConfig{Links: "[bad"}).Validate(); err == nil {
		t.Fatal("invalid selector accepted")
	}
}

func TestCareerDeadlineKeepsFutureInstants(t *testing.T) {
	raw := "2035-11-05T17:59:59Z"
	at := careerDeadline(raw)
	if at == nil || at.Format("2006-01-02T15:04:05Z07:00") != raw {
		t.Fatal("future deadline was dropped")
	}
	if careerDeadline("2035-11-05").Day() != 6 {
		t.Fatal("date-only deadline is not inclusive")
	}
}

func TestCompanyCareerStructuredFieldsUseExistingSchemaMapping(t *testing.T) {
	raw := []byte(`{"@type":"JobPosting","title":"Python Engineer","description":"Build services","url":"https://example.test/job/1","datePosted":"2020-01-01T12:00:00Z","validThrough":"2035-11-05T17:59:59Z","hiringOrganization":{"name":"Actual BD Employer"},"jobLocation":{"address":{"addressLocality":"Dhaka","addressCountry":"BD"}},"employmentType":"FULL_TIME"}`)
	job, err := careerLD(raw, CompanyEntry{Company: "Board label"}, "https://example.test/job/1", "")
	if err != nil || job.Company != "Actual BD Employer" || job.Location == "" || len(job.Countries) != 1 || job.Countries[0] != "bd" || job.ApplicationDeadline == nil || job.PostedAt == nil || job.EmploymentType != "full_time" {
		t.Fatalf("structured source facts lost: %+v %v", job, err)
	}
}
