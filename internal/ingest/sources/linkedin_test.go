package sources

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

const linkedInTestDetail = `<h2 class="top-card-layout__title">Python Engineer</h2><a class="topcard__org-name-link">Example BD</a><span class="topcard__flavor--bullet">Dhaka, Bangladesh</span><div class="description__text"><div class="show-more-less-html__markup"><p>Build Python services and data tools.</p><script>bad()</script></div></div><li class="description__job-criteria-item"><h3 class="description__job-criteria-subheader">Employment type</h3><span class="description__job-criteria-text">Full-time</span></li>`

type linkedInTestHTTP func(context.Context, string) (*html.Node, error)

func (f linkedInTestHTTP) GetHTML(ctx context.Context, u string) (*html.Node, error) {
	return f(ctx, u)
}
func linkedInTestNode(body string) *html.Node { n, _ := html.Parse(strings.NewReader(body)); return n }
func linkedInTestCard(id string) string {
	return fmt.Sprintf(`<div data-entity-urn="urn:li:jobPosting:%s"><a class="base-card__full-link" href="https://bd.linkedin.com/jobs/view/python-engineer-%s?trk=test"></a><h3 class="base-search-card__title">Python Engineer</h3><h4 class="base-search-card__subtitle">Example BD</h4><span class="job-search-card__location">Dhaka, Bangladesh</span><time datetime="2026-11-05"></time></div>`, id, id)
}

func TestLinkedInIdentity(t *testing.T) {
	for _, raw := range []string{"https://www.linkedin.com/jobs/view/4487731921/", "https://bd.linkedin.com/jobs/view/python-engineer-4487731921?trk=copy#details", "https://linkedin.com/jobs/view/4487731921"} {
		if got := LinkedInJobID(raw); got != "4487731921" {
			t.Errorf("%s => %s", raw, got)
		}
	}
	for _, raw := range []string{"http://linkedin.com/jobs/view/1", "https://linkedin.com.evil.test/jobs/view/1", "https://evil.linkedin.com/jobs/view/1", "https://u:p@linkedin.com/jobs/view/1", "https://linkedin.com:444/jobs/view/1", "https://linkedin.com/jobs/search/?currentJobId=1", "https://linkedin.com/jobs/view/0", "https://linkedin.com/jobs/view/01", "https://linkedin.com/in/person"} {
		if LinkedInJobID(raw) != "" {
			t.Errorf("accepted %s", raw)
		}
	}
}

func TestLinkedInDetailSignals(t *testing.T) {
	job := ParseLinkedInJob(linkedInTestNode(linkedInTestDetail+`<footer>No longer accepting applications</footer>`), "123")
	if job.Unreadable || job.Removed || job.Title != "Python Engineer" || job.Company != "Example BD" || job.EmploymentType != "full_time" || strings.Contains(job.Description, "script") || strings.Contains(job.Description, "bad()") {
		t.Fatalf("bad parsed detail: %+v", job)
	}
	for _, body := range []string{"<h1>Sign in to LinkedIn</h1>", "<div>checkpoint</div>", strings.ReplaceAll(linkedInTestDetail, "description__text", "changed-description")} {
		j := ParseLinkedInJob(linkedInTestNode(body), "123")
		if !j.Unreadable || j.Removed {
			t.Fatal("unreadable detail became closure or valid posting")
		}
	}
	j := ParseLinkedInJob(linkedInTestNode(`<span class="closed-job__flavor">No longer accepting applications</span>`), "123")
	if !j.Removed || j.Unreadable {
		t.Fatal("explicit closure lost")
	}
	if !ParseLinkedInJob(nil, "123").Unreadable {
		t.Fatal("nil detail accepted")
	}
}

func TestLinkedInBoundedCollection(t *testing.T) {
	t.Setenv("LINKEDIN_LOCATION", "")
	t.Setenv("LINKEDIN_KEYWORDS", "")
	t.Setenv("LINKEDIN_MAX_PAGES", "")
	ctx := context.Background()
	details := map[string]int{}
	pages := 0
	getter := linkedInTestHTTP(func(_ context.Context, raw string) (*html.Node, error) {
		u, _ := url.Parse(raw)
		if strings.HasPrefix(raw, linkedInSearch) {
			if u.Query().Get("location") != "Worldwide" {
				t.Fatal("worldwide default lost")
			}
			pages++
			id := "123"
			if u.Query().Get("start") == "10" {
				id = "456"
			}
			return linkedInTestNode(linkedInTestCard(id)), nil
		}
		id := strings.TrimPrefix(raw, LinkedInDetailURL)
		details[id]++
		return linkedInTestNode(linkedInTestDetail), nil
	})
	jobs, err := (linkedin{http: getter}).Fetch(ctx, CompanyEntry{})
	if err != nil || len(jobs) != 2 || pages != len(linkedInQueries)*linkedInPages || details["123"] != 1 || details["456"] != 1 {
		t.Fatalf("collection/dedup: %d %d %+v %v", len(jobs), pages, details, err)
	}
	if jobs[0].PostedAt == nil || jobs[0].PostedAt.Format("2006-01-02") != "2026-11-05" || jobs[0].URL != "https://www.linkedin.com/jobs/view/123/" {
		t.Fatal("provenance/date lost")
	}
	reg := map[string]Source{"linkedin": NewLinkedIn(nil)}
	if !slices.Contains(SelfClosingProviders(reg), "linkedin") || FullBoardListingProviders(reg)["linkedin"] || len(FullCatalogProviders(reg)) != 0 {
		t.Fatal("bounded discovery must never earn absence-based closure")
	}
	if !slices.Contains(AggregatorProviders(reg), "linkedin") {
		t.Fatal("missing aggregator classification")
	}
}

func TestLinkedInAnchorCardLayout(t *testing.T) {
	card := `<a href="https://bd.linkedin.com/jobs/view/python-ai-lead-123?trk=search" data-entity-urn="urn:li:jobPosting:123"><h3 class="base-search-card__title">Python AI Lead</h3><h4 class="base-search-card__subtitle">A Renowned Software Firm</h4><span class="job-search-card__location">Dhaka, Bangladesh</span><time datetime="2026-11-05"></time></a>`
	jobs, err := linkedInCards(linkedInTestNode(card))
	if err != nil || len(jobs) != 1 || jobs[0].ExternalID != "123" {
		t.Fatalf("anchor card: %+v %v", jobs, err)
	}
}

func TestLinkedInFailureAndClosure(t *testing.T) {
	t.Setenv("LINKEDIN_LOCATION", "")
	t.Setenv("LINKEDIN_KEYWORDS", "")
	t.Setenv("LINKEDIN_MAX_PAGES", "")
	for _, mode := range []string{"listing-failure", "empty", "login", "detail-refusal", "detail-unreadable"} {
		t.Run(mode, func(t *testing.T) {
			getter := linkedInTestHTTP(func(_ context.Context, raw string) (*html.Node, error) {
				if strings.HasPrefix(raw, linkedInSearch) {
					switch mode {
					case "listing-failure":
						return nil, errors.New("unavailable")
					case "empty":
						return linkedInTestNode(""), nil
					case "login":
						return linkedInTestNode("<h1>Sign in</h1>"), nil
					}
					return linkedInTestNode(linkedInTestCard("123")), nil
				}
				switch mode {
				case "detail-refusal":
					return nil, &StatusError{Code: 429}
				case "detail-unreadable":
					return linkedInTestNode("<h1>Sign in</h1>"), nil
				}
				return nil, errors.New("unexpected")
			})
			if _, err := (linkedin{http: getter}).Fetch(context.Background(), CompanyEntry{}); err == nil {
				t.Fatal("incomplete or inaccessible crawl succeeded")
			}
		})
	}
	for _, code := range []int{404, 410, 403, 429, 500} {
		j, err := FetchLinkedInJob(context.Background(), linkedInTestHTTP(func(context.Context, string) (*html.Node, error) {
			return nil, &StatusError{Code: code, FinalURL: LinkedInDetailURL + "123"}
		}), "123")
		if code == 404 || code == 410 {
			if err != nil || !j.Removed {
				t.Fatal("explicit gone not removed")
			}
		} else if err == nil || j.Removed {
			t.Fatal("refusal became closure")
		}
	}
}

func TestLinkedInWorldwideAndConfig(t *testing.T) {
	t.Setenv("LINKEDIN_LOCATION", "")
	t.Setenv("LINKEDIN_KEYWORDS", "")
	t.Setenv("LINKEDIN_MAX_PAGES", "")
	location, _, pages, err := linkedInConfig()
	if err != nil || location != "Worldwide" || pages != 2 {
		t.Fatal("worldwide defaults lost")
	}
	j := ParseLinkedInJob(linkedInTestNode(strings.ReplaceAll(linkedInTestDetail, "Dhaka, Bangladesh", "London, United Kingdom")), "123")
	if j.Unreadable || j.Location != "London, United Kingdom" {
		t.Fatal("non-Bangladesh job rejected")
	}
	t.Setenv("LINKEDIN_LOCATION", "Bangladesh")
	t.Setenv("LINKEDIN_KEYWORDS", `["python developer","data analyst"]`)
	t.Setenv("LINKEDIN_MAX_PAGES", "3")
	location, queries, pages, err := linkedInConfig()
	if err != nil || location != "Bangladesh" || len(queries) != 2 || pages != 3 {
		t.Fatalf("configuration lost: %s %v %d %v", location, queries, pages, err)
	}
	for _, tc := range []struct{ key, value string }{{"LINKEDIN_KEYWORDS", "[]"}, {"LINKEDIN_KEYWORDS", `[""]`}, {"LINKEDIN_KEYWORDS", "python"}, {"LINKEDIN_MAX_PAGES", "0"}, {"LINKEDIN_MAX_PAGES", "6"}, {"LINKEDIN_MAX_PAGES", "bad"}} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, _, _, err := linkedInConfig(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestLinkedInUnreadableBatchStops(t *testing.T) {
	t.Setenv("LINKEDIN_LOCATION", "")
	t.Setenv("LINKEDIN_KEYWORDS", `["python"]`)
	t.Setenv("LINKEDIN_MAX_PAGES", "1")
	requests := 0
	getter := linkedInTestHTTP(func(_ context.Context, raw string) (*html.Node, error) {
		if strings.HasPrefix(raw, linkedInSearch) {
			return linkedInTestNode(linkedInTestCard("123") + linkedInTestCard("456") + linkedInTestCard("789") + linkedInTestCard("999")), nil
		}
		requests++
		return linkedInTestNode("<h1>Sign in</h1>"), nil
	})
	_, err := (linkedin{http: getter}).Fetch(context.Background(), CompanyEntry{})
	if err == nil || requests != 3 {
		t.Fatalf("access wall not stopped: %d %v", requests, err)
	}
}

func TestLinkedInRedirectedRemovalIsUnreadable(t *testing.T) {
	getter := linkedInTestHTTP(func(context.Context, string) (*html.Node, error) {
		return nil, &StatusError{Code: 404, FinalURL: "https://www.linkedin.com/authwall"}
	})
	j, err := FetchLinkedInJob(context.Background(), getter, "123")
	if err == nil || j.Removed {
		t.Fatal("redirected auth-wall 404 became closure")
	}
}
