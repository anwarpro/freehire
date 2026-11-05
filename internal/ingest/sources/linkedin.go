package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/strelov1/freehire/internal/platform/htmltext"
)

const linkedInSearch = "https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search"
const LinkedInDetailURL = "https://www.linkedin.com/jobs-guest/jobs/api/jobPosting/"
const linkedInPages = 2

// This is a bounded discovery slice, not the complete LinkedIn catalogue. A
// posting drifting below these pages is never evidence that it has closed.
var linkedInQueries = []string{"software engineer", "frontend developer", "backend developer", "python", "data analyst", "machine learning"}

type linkedin struct{ http HTMLGetter }

func NewLinkedIn(c HTMLGetter) Source { return linkedin{http: pacedHTMLGetter(c, 2*time.Second, 1)} }
func (linkedin) Provider() string     { return "linkedin" }
func (linkedin) boardless()           {}
func (linkedin) aggregator()          {}

// Only explicit detail removal closes a posting. The liveness worker checks
// older jobs outside the discovery window using the same guest detail parser.
func (linkedin) selfClosing() {}

var linkedInPath = regexp.MustCompile(`^/jobs/view/(?:[^/]*-)?([1-9][0-9]{0,18})/?$`)
var linkedInHost = regexp.MustCompile(`^(?:www\.|[a-z]{2}\.)?linkedin\.com$`)

// LinkedInJobID normalizes numeric and title-bearing public job links. Search,
// profile, redirector, credentialed and nonstandard-port URLs are not job links.
func LinkedInJobID(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || !linkedInHost.MatchString(strings.ToLower(u.Hostname())) {
		return ""
	}
	m := linkedInPath.FindStringSubmatch(u.Path)
	if m == nil {
		return ""
	}
	return m[1]
}

func (s linkedin) Fetch(ctx context.Context, _ CompanyEntry) ([]Job, error) {
	location, queries, pages, err := linkedInConfig()
	if err != nil {
		return nil, err
	}
	var listed []Job
	seen := map[string]bool{}
	for _, query := range queries {
		for page := 0; page < pages; page++ {
			params := url.Values{"keywords": {query}, "location": {location}, "start": {strconv.Itoa(page * 10)}}
			node, err := s.http.GetHTML(ctx, linkedInSearch+"?"+params.Encode())
			if err != nil {
				return nil, fmt.Errorf("linkedin search %q page %d: %w", query, page+1, err)
			}
			cards, err := linkedInCards(node)
			if err != nil {
				return nil, fmt.Errorf("linkedin search %q: %w", query, err)
			}
			if len(cards) == 0 {
				break
			}
			for _, card := range cards {
				if !seen[card.ExternalID] {
					listed = append(listed, card)
					seen[card.ExternalID] = true
					if len(listed) > 300 {
						return nil, fmt.Errorf("linkedin: discovery exceeded the 300-job run budget")
					}
				}
			}
		}
	}
	if len(listed) == 0 {
		return nil, fmt.Errorf("linkedin: every configured query returned no readable postings")
	}
	out := make([]Job, 0, len(listed))
	readable := 0
	consecutiveUnreadable := 0
	unreadable := func(card Job) error {
		card.Unreadable = true
		out = append(out, card)
		consecutiveUnreadable++
		if consecutiveUnreadable >= 3 {
			return fmt.Errorf("linkedin: three consecutive details unreadable; deferring remaining jobs")
		}
		return nil
	}
	for _, card := range listed {
		j, err := FetchLinkedInJob(ctx, s.http, card.ExternalID)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err != nil {
			var status *StatusError
			if errors.As(err, &status) && (status.Code == 403 || status.Code == 429 || status.Code == 999) {
				return nil, fmt.Errorf("linkedin detail access refused: %w", err)
			}
			if err := unreadable(card); err != nil {
				return nil, err
			}
			continue
		}
		if j.PostedAt == nil {
			j.PostedAt = card.PostedAt
		}
		if j.Unreadable {
			if err := unreadable(card); err != nil {
				return nil, err
			}
			continue
		}
		consecutiveUnreadable = 0
		readable++
		out = append(out, j)
	}
	if readable == 0 {
		return nil, fmt.Errorf("linkedin: none of %d listed jobs had readable details", len(listed))
	}
	return out, nil
}

func linkedInCards(root *html.Node) ([]Job, error) {
	var cards []Job
	var malformed bool
	walk(root, func(n *html.Node) bool {
		urn := Attr(n, "data-entity-urn")
		if !strings.HasPrefix(urn, "urn:li:jobPosting:") {
			return true
		}
		id := strings.TrimPrefix(urn, "urn:li:jobPosting:")
		link := ElementAttr(n, "a", "base-card__full-link", "href")
		if n.Data == "a" {
			link = Attr(n, "href")
		}
		if LinkedInJobID(link) != id {
			malformed = true
			return false
		}
		j := Job{ExternalID: id, URL: "https://www.linkedin.com/jobs/view/" + id + "/", Title: linkedInText(n, "base-search-card__title"), Company: linkedInText(n, "base-search-card__subtitle"), Location: linkedInText(n, "job-search-card__location"), PostedAt: ParseDate(ElementAttr(n, "time", "", "datetime"))}
		if j.Title == "" || j.Company == "" || j.Location == "" {
			malformed = true
			return false
		}
		cards = append(cards, j)
		return false
	})
	if malformed {
		return nil, fmt.Errorf("job-card markup or identity changed")
	}
	// Empty fragments are how the guest endpoint signals the end. Nonempty
	// documents with no cards are a login/challenge/markup failure, not success.
	if len(cards) == 0 && strings.TrimSpace(textContent(root)) != "" {
		return nil, fmt.Errorf("no readable job cards; access or markup changed")
	}
	return cards, nil
}

// FetchLinkedInJob reads one public guest detail. Used by crawl and link intake;
// 404/410 are explicit removal, while refusal and malformed bodies remain errors
// or Unreadable and must never be interpreted as closure.
func FetchLinkedInJob(ctx context.Context, c HTMLGetter, id string) (Job, error) {
	if LinkedInJobID("https://www.linkedin.com/jobs/view/"+id+"/") != id || id == "" {
		return Job{}, fmt.Errorf("linkedin: invalid job id")
	}
	requested := LinkedInDetailURL + id
	node, err := c.GetHTML(ctx, requested)
	if err != nil {
		var status *StatusError
		if errors.As(err, &status) && (status.Code == 404 || status.Code == 410) && status.FinalURL == requested {
			return Job{ExternalID: id, URL: "https://www.linkedin.com/jobs/view/" + id + "/", Removed: true}, nil
		}
		return Job{}, err
	}
	return ParseLinkedInJob(node, id), nil
}

// ParseLinkedInJob deliberately ignores recommendation cards, login-wall prose
// and footer mentions of expiration. Only the job's closed marker is evidence.
func ParseLinkedInJob(node *html.Node, id string) Job {
	j := Job{ExternalID: id, URL: "https://www.linkedin.com/jobs/view/" + id + "/"}
	if node == nil {
		j.Unreadable = true
		return j
	}
	if strings.EqualFold(strings.TrimSpace(linkedInText(node, "closed-job__flavor")), "No longer accepting applications") {
		j.Removed = true
		return j
	}
	j.Title = linkedInText(node, "top-card-layout__title")
	j.Company = linkedInText(node, "topcard__org-name-link")
	j.Location = linkedInText(node, "topcard__flavor--bullet")
	desc := linkedInNode(linkedInNode(node, "description__text"), "show-more-less-html__markup")
	if desc != nil {
		j.Description = sanitizeHTML(innerHTML(desc))
	}
	if j.Title == "" || j.Company == "" || j.Location == "" || strings.TrimSpace(htmltext.ToText(j.Description)) == "" {
		j.Unreadable = true
		return j
	}
	walk(node, func(n *html.Node) bool {
		if !HasClass(n, "description__job-criteria-item") {
			return true
		}
		label := strings.ToLower(linkedInText(n, "description__job-criteria-subheader"))
		value := strings.ToLower(linkedInText(n, "description__job-criteria-text"))
		if label == "employment type" {
			switch value {
			case "full-time":
				j.EmploymentType = "full_time"
			case "part-time":
				j.EmploymentType = "part_time"
			case "contract":
				j.EmploymentType = "contract"
			case "internship":
				j.EmploymentType = "internship"
			}
		}
		return false
	})
	return j
}

func linkedInNode(root *html.Node, class string) *html.Node {
	if root == nil {
		return nil
	}
	return firstByClass(root, class)
}
func linkedInText(root *html.Node, class string) string {
	node := linkedInNode(root, class)
	if node == nil {
		return ""
	}
	return textContent(node)
}

// Search is worldwide by default. Location is a LinkedIn search hint, never an
// eligibility assertion: preserve each posting's actual location unchanged.
func linkedInConfig() (string, []string, int, error) {
	location := strings.TrimSpace(os.Getenv("LINKEDIN_LOCATION"))
	if location == "" {
		location = "Worldwide"
	}
	if len(location) > 200 {
		return "", nil, 0, fmt.Errorf("linkedin: LINKEDIN_LOCATION exceeds 200 bytes")
	}
	queries := append([]string(nil), linkedInQueries...)
	if raw := strings.TrimSpace(os.Getenv("LINKEDIN_KEYWORDS")); raw != "" {
		if json.Unmarshal([]byte(raw), &queries) != nil || len(queries) == 0 || len(queries) > 12 {
			return "", nil, 0, fmt.Errorf("linkedin: LINKEDIN_KEYWORDS must be a JSON array of 1–12 search terms")
		}
		for i, q := range queries {
			queries[i] = strings.TrimSpace(q)
			if queries[i] == "" || len(queries[i]) > 200 {
				return "", nil, 0, fmt.Errorf("linkedin: search terms must contain 1–200 bytes")
			}
		}
	}
	pages := linkedInPages
	if raw := strings.TrimSpace(os.Getenv("LINKEDIN_MAX_PAGES")); raw != "" {
		var err error
		pages, err = strconv.Atoi(raw)
		if err != nil || pages < 1 || pages > 5 {
			return "", nil, 0, fmt.Errorf("linkedin: LINKEDIN_MAX_PAGES must be 1–5")
		}
	}
	if len(queries)*pages > 30 {
		return "", nil, 0, fmt.Errorf("linkedin: configured search exceeds 30 listing pages per run")
	}
	return location, queries, pages, nil
}
