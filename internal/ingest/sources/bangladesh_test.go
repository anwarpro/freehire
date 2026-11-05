package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type bangladeshFixture struct {
	careerFixture
	json map[string]string
}

func (f bangladeshFixture) GetJSON(_ context.Context, u string, v any) error {
	body, ok := f.json[u]
	if !ok {
		return fmt.Errorf("missing public feed")
	}
	return json.Unmarshal([]byte(body), v)
}
func TestVivasoftUsesOpenLinksAndExactDeadline(t *testing.T) {
	fixture := bangladeshFixture{careerFixture: careerFixture{"https://vivasoftltd.com/career/": `<a href="https://app.novara.com.my/jobs/python">Apply Now</a><a href="https://app.novara.com.my/jobs/closed">Closed</a>`}, json: map[string]string{"https://api.app.novara.com.my/api/job-builder/v1/public/jobs/slug/python": `{"success":true,"data":{"id":"1","title":"Python Engineer","company_name":"Vivasoft Limited","description":"<p>Build Django APIs</p>","status":"published","posted_at":"2020-01-01T12:00:00Z","deadline":"2035-11-05T17:59:59Z","location":{"city":"Dhaka","country":"Bangladesh"}}}`}}
	e := CompanyEntry{Provider: "vivasoft", Company: "Vivasoft Limited", Board: "vivasoftltd.com"}
	jobs, err := NewVivasoft(fixture).Fetch(context.Background(), e)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("fetch: %+v %v", jobs, err)
	}
	if jobs[0].ApplicationDeadline == nil || jobs[0].ApplicationDeadline.Year() != 2035 || jobs[0].PostedAt == nil || jobs[0].Removed {
		t.Fatal("source dates or status were lost")
	}
	fixture.careerFixture["https://vivasoftltd.com/career/"] = `<a href="https://app.novara.com.my/jobs/closed">Closed</a>`
	jobs, err = NewVivasoft(fixture).Fetch(context.Background(), e)
	if err != nil || len(jobs) != 0 {
		t.Fatal("explicitly closed-only listing was not empty")
	}
	fixture.careerFixture["https://vivasoftltd.com/career/"] = `<main>Checking browser</main>`
	if _, err := NewVivasoft(fixture).Fetch(context.Background(), e); err == nil {
		t.Fatal("challenge counted as empty")
	}
}
func TestCefaloExpiredFeedNeverFetchesOrRepublishesBody(t *testing.T) {
	fixture := bangladeshFixture{json: map[string]string{"https://ats.cefalo.com/api/v1/public/jobs": `[{"id":35,"title":"Python Lead","slug":"python-lead-35","isExpired":true,"deadline":"2020-01-01T00:00:00Z"}]`}}
	jobs, err := NewCefalo(fixture).Fetch(context.Background(), CompanyEntry{Provider: "cefalo", Board: "career.cefalo.com"})
	if err != nil || len(jobs) != 1 || !jobs[0].Removed {
		t.Fatalf("expired job republished: %+v %v", jobs, err)
	}
	if !strings.Contains(jobs[0].URL, "python-lead-35") {
		t.Fatal("original identity lost")
	}
}
func TestPinpointCustomCompanyHostAndEmptyProof(t *testing.T) {
	fixture := &fakeHTTP{body: `{"data":[]}`}
	jobs, err := NewPinpoint(fixture).Fetch(context.Background(), CompanyEntry{Board: "careers.enosisbd.com"})
	if err != nil || len(jobs) != 0 || fixture.gotURL != "https://careers.enosisbd.com/postings.json" {
		t.Fatalf("custom host: %+v %v %s", jobs, err, fixture.gotURL)
	}
	fixture.body = `{}`
	if _, err := NewPinpoint(fixture).Fetch(context.Background(), CompanyEntry{Board: "careers.enosisbd.com"}); err == nil {
		t.Fatal("missing feed counted as empty")
	}
}
