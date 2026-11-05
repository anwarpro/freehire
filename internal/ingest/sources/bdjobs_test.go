package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

type bdjobsFixture struct {
	pages    map[string]string
	details  map[string]string
	requests []string
}

func (f *bdjobsFixture) GetJSON(ctx context.Context, raw string, out any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.requests = append(f.requests, raw)
	u, _ := url.Parse(raw)
	var body string
	if strings.HasSuffix(u.Path, "/GetJobSearch") {
		if u.Query().Get("category") != "8" || u.Query().Get("rpp") != "50" || u.Query().Get("ToggleJobs") != "true" {
			return fmt.Errorf("wrong search scope")
		}
		body = f.pages[u.Query().Get("pg")]
	} else {
		body = f.details[u.Query().Get("jobId")]
	}
	if body == "" {
		return fmt.Errorf("upstream unavailable")
	}
	return json.Unmarshal([]byte(body), out)
}

func bdjobsTestPage(start, count, total, pages int) string {
	rows := []map[string]any{}
	for id := start; id < start+count; id++ {
		rows = append(rows, map[string]any{"Jobid": strconv.Itoa(id), "jobTitle": "Software Engineer", "companyName": "Example BD", "publishDate": "2020-05-01T06:00:00Z", "deadlineDB": "2035-11-05T00:00:00Z"})
	}
	b, _ := json.Marshal(map[string]any{"statuscode": "1", "data": rows, "premiumData": []any{}, "common": map[string]int{"total_records_found": total, "totalpages": pages}})
	return string(b)
}

func bdjobsTestDetail(id int) string {
	return fmt.Sprintf(`{"statuscode":"0","data":[{"JobId":"%d","JobFound":"True","error":"0","CompnayName":"Example BD","JobTitle":"সফটওয়্যার প্রকৌশলী / Software Engineer","JobDescription":"<p>Build Python APIs.</p><script>evil()</script>","AdditionJobRequirements":"<p>Django and PostgreSQL</p>","EducationRequirements":"<p>BSc in CSE</p>","JobLocation":"Dhaka","JobNature":"Full Time","JobWorkPlace":"Work from home","DeadlineDB":"2035-11-06","Closed":0}]}`, id)
}

func TestBDJobsCompletePaginationAndSourceFacts(t *testing.T) {
	f := &bdjobsFixture{pages: map[string]string{"1": bdjobsTestPage(1, 50, 51, 2), "2": bdjobsTestPage(51, 1, 51, 2)}, details: map[string]string{}}
	for id := 1; id <= 51; id++ {
		f.details[strconv.Itoa(id)] = bdjobsTestDetail(id)
	}
	// Promoted copies must not produce a second posting or detail request.
	f.pages["1"] = strings.Replace(f.pages["1"], `"premiumData":[]`, `"premiumData":[{"Jobid":"51","jobTitle":"Software Engineer","companyName":"Example BD"}]`, 1)
	jobs, err := NewBDJobs(f).Fetch(context.Background(), CompanyEntry{})
	if err != nil || len(jobs) != 51 {
		t.Fatalf("fetch: %d jobs, %v", len(jobs), err)
	}
	if len(f.requests) != 53 {
		t.Fatalf("duplicate or missing requests: %d", len(f.requests))
	}
	for _, j := range jobs {
		if j.Company != "Example BD" || j.URL != "https://bdjobs.com/h/details/"+j.ExternalID || j.PostedAt == nil || j.PostedAt.Format("2006-01-02") != "2020-05-01" || j.ApplicationDeadline == nil || j.ApplicationDeadline.Format("2006-01-02T15:04:05Z") != "2035-11-06T18:00:00Z" {
			t.Fatalf("lost source facts: %+v", j)
		}
		if !j.IsTechHint || !j.Remote || j.WorkMode != "remote" || j.EmploymentType != "full_time" || !strings.Contains(j.Description, "PostgreSQL") || strings.Contains(j.Description, "evil") || j.Unreadable {
			t.Fatalf("wrong body/facets: %+v", j)
		}
	}
}

func TestBDJobsRejectsIncompleteOrUnverifiedListings(t *testing.T) {
	for name, pages := range map[string]map[string]string{
		"missing metadata": {"1": `{"statuscode":"1","data":[]}`},
		"null data":        {"1": `{"statuscode":"1","data":null,"common":{"total_records_found":0,"totalpages":0}}`},
		"api error":        {"1": `{"statuscode":"0","data":[],"common":{"total_records_found":0,"totalpages":0}}`},
		"short first page": {"1": bdjobsTestPage(1, 49, 51, 2)},
		"later failure":    {"1": bdjobsTestPage(1, 50, 51, 2)},
		"repeated page":    {"1": bdjobsTestPage(1, 50, 100, 2), "2": bdjobsTestPage(1, 50, 100, 2)},
		"changing total":   {"1": bdjobsTestPage(1, 50, 51, 2), "2": bdjobsTestPage(51, 2, 52, 2)},
		"page cap":         {"1": bdjobsTestPage(1, 50, 5001, 101)},
		"bad ID":           {"1": strings.Replace(bdjobsTestPage(1, 1, 1, 1), `"Jobid":"1"`, `"Jobid":"../x"`, 1)},
	} {
		t.Run(name, func(t *testing.T) {
			f := &bdjobsFixture{pages: pages}
			if _, err := NewBDJobs(f).Fetch(context.Background(), CompanyEntry{}); err == nil {
				t.Fatal("unsafe listing accepted")
			}
			for _, u := range f.requests {
				if strings.Contains(u, "jobDetails") {
					t.Fatal("details fetched before completeness proof")
				}
			}
		})
	}
}

func TestBDJobsExplicitEmptyListing(t *testing.T) {
	for _, page := range []string{bdjobsTestPage(1, 0, 0, 0), bdjobsTestPage(1, 0, 0, 1)} {
		jobs, err := NewBDJobs(&bdjobsFixture{pages: map[string]string{"1": page}}).Fetch(context.Background(), CompanyEntry{})
		if err != nil || len(jobs) != 0 {
			t.Fatalf("verified empty: %v %+v", err, jobs)
		}
	}
}

func TestBDJobsUnreadableDetailsAndExplicitRemoval(t *testing.T) {
	for name, body := range map[string]string{
		"network failure":     "",
		"malformed success":   `{"statuscode":"0","data":[]}`,
		"wrong identity":      bdjobsTestDetail(99),
		"image only":          strings.ReplaceAll(strings.Replace(bdjobsTestDetail(2), `<p>Build Python APIs.</p><script>evil()</script>`, `<img src="https://example.test/job.png">`, 1), `<p>Django and PostgreSQL</p>`, ""),
		"locked early access": `{"statuscode":"2","message":"early access","data":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			f := &bdjobsFixture{pages: map[string]string{"1": bdjobsTestPage(1, 2, 2, 1)}, details: map[string]string{"1": bdjobsTestDetail(1), "2": body}}
			jobs, err := NewBDJobs(f).Fetch(context.Background(), CompanyEntry{})
			if err != nil || len(jobs) != 2 || !jobs[1].Unreadable || jobs[1].Removed {
				t.Fatalf("unreadable silently removed: %+v %v", jobs, err)
			}
			f.details["1"] = ""
			if _, err := NewBDJobs(f).Fetch(context.Background(), CompanyEntry{}); err == nil {
				t.Fatal("wholly unreadable crawl succeeded")
			}
		})
	}
	for name, body := range map[string]string{
		"gone":    `{"statuscode":"1","message":"data not found","data":null}`,
		"closed":  strings.Replace(bdjobsTestDetail(1), `"Closed":0`, `"Closed":1`, 1),
		"expired": strings.Replace(bdjobsTestDetail(1), `2035-11-06`, `2020-01-01`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			f := &bdjobsFixture{pages: map[string]string{"1": bdjobsTestPage(1, 1, 1, 1)}, details: map[string]string{"1": body}}
			jobs, err := NewBDJobs(f).Fetch(context.Background(), CompanyEntry{})
			if err != nil || len(jobs) != 1 || !jobs[0].Removed || jobs[0].Unreadable {
				t.Fatalf("explicit removal lost: %+v %v", jobs, err)
			}
		})
	}
}

func TestBDJobsDetailCanExtendStaleListingDeadline(t *testing.T) {
	f := &bdjobsFixture{pages: map[string]string{"1": strings.ReplaceAll(bdjobsTestPage(1, 1, 1, 1), "2035-11-05", "2020-01-01")}, details: map[string]string{"1": bdjobsTestDetail(1)}}
	jobs, err := NewBDJobs(f).Fetch(context.Background(), CompanyEntry{})
	if err != nil || len(jobs) != 1 || jobs[0].Removed || jobs[0].Unreadable || len(f.requests) != 2 {
		t.Fatalf("stale listing overrode the current deadline: %+v %v", jobs, err)
	}
}

func TestBDJobsDeadlineIsBangladeshEndOfDay(t *testing.T) {
	for _, raw := range []string{"2035-11-05", "2035-11-05T00:00:00Z"} {
		d := bdjobsDeadline(raw)
		if d == nil || d.Format("2006-01-02T15:04:05Z") != "2035-11-05T18:00:00Z" {
			t.Fatalf("wrong inclusive deadline: %s %v", raw, d)
		}
	}
	for _, raw := range []string{"", "2035-02-30", "2035-11-05junk", "2035-11-05T12:00:00Z"} {
		if bdjobsDeadline(raw) != nil {
			t.Fatalf("accepted invalid calendar deadline %s", raw)
		}
	}
}

func TestBDJobsRegistrySchedulingAndTaxonomy(t *testing.T) {
	registry := Taxonomy()
	if registry["bdjobs"] == nil || !FullBoardListingProviders(registry)["bdjobs"] {
		t.Fatal("collector missing from registry/completeness taxonomy")
	}
	for _, key := range BoardKeyedProviders(registry) {
		if key == "bdjobs" {
			t.Fatal("bootstrap cannot initialize boardless BDJobs")
		}
	}
	found := false
	for _, key := range AggregatorProviders(registry) {
		found = found || key == "bdjobs"
	}
	if !found {
		t.Fatal("BDJobs must participate in aggregator dedup")
	}
}

// BDJobs' own row total counts repeated IDs in adjacent regular pages. Those
// overlaps must not make an otherwise complete public listing permanently fail.
func TestBDJobsAdjacentPageOverlapKeepsOneNativeIdentity(t *testing.T) {
	f := &bdjobsFixture{pages: map[string]string{"1": bdjobsTestPage(1, 50, 52, 2), "2": bdjobsTestPage(50, 2, 52, 2)}, details: map[string]string{}}
	for id := 1; id <= 51; id++ {
		f.details[strconv.Itoa(id)] = bdjobsTestDetail(id)
	}
	jobs, err := NewBDJobs(f).Fetch(context.Background(), CompanyEntry{})
	if err != nil || len(jobs) != 51 {
		t.Fatalf("valid overlap rejected or duplicated: %d %v", len(jobs), err)
	}
}

func TestBDJobsCancelledCrawlFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewBDJobs(&bdjobsFixture{}).Fetch(ctx, CompanyEntry{}); err == nil {
		t.Fatal("cancelled crawl reported success")
	}
}

func TestBDJobsRoleBodyCanBeInRequirements(t *testing.T) {
	// A real StorePilot advert puts its .NET responsibilities in the requirements
	// field and leaves JobDescription as <p></p>. It is a readable posting.
	body := strings.Replace(bdjobsTestDetail(1), `<p>Build Python APIs.</p><script>evil()</script>`, `<p></p>`, 1)
	f := &bdjobsFixture{pages: map[string]string{"1": bdjobsTestPage(1, 1, 1, 1)}, details: map[string]string{"1": body}}
	jobs, err := NewBDJobs(f).Fetch(context.Background(), CompanyEntry{})
	if err != nil || len(jobs) != 1 || jobs[0].Unreadable || !strings.Contains(jobs[0].Description, "PostgreSQL") {
		t.Fatalf("readable requirements discarded: %+v %v", jobs, err)
	}
}
