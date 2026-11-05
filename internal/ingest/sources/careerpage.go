package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

// CareerPageConfig describes a server-rendered career listing. Empty selectors use
// schema.org JobPosting. Selectors are curator configuration, never executable code.
type CareerPageConfig struct {
	Links           string   `json:"links,omitempty"`
	Title           string   `json:"title,omitempty"`
	Body            string   `json:"body,omitempty"`
	Location        string   `json:"location,omitempty"`
	Posted          string   `json:"posted,omitempty"`
	Next            string   `json:"next,omitempty"`
	EmptyText       string   `json:"empty_text,omitempty"`
	DetailHosts     []string `json:"detail_hosts,omitempty"`
	DefaultLocation string   `json:"default_location,omitempty"`
}

// CareerPageURL canonicalizes a crawl identity without discarding meaningful query parameters.
func CareerPageURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || (u.Port() != "" && u.Port() != "443") || len(raw) > 2048 || strings.ContainsAny(raw, "\x00\r\n\t") {
		return "", fmt.Errorf("career page must be an absolute HTTPS URL without credentials")
	}
	u.Host = strings.ToLower(u.Hostname())
	u.Fragment = ""
	q := u.Query()
	for k := range q {
		if strings.HasPrefix(k, "utm_") {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c CareerPageConfig) Validate() error {
	for _, s := range []string{c.Links, c.Title, c.Body, c.Location, c.Posted, c.Next} {
		if len(s) > 512 {
			return fmt.Errorf("selector exceeds 512 characters")
		}
		if s != "" {
			if _, err := cascadia.Parse(s); err != nil {
				return fmt.Errorf("invalid CSS selector: %w", err)
			}
		}
	}
	if len(c.EmptyText) > 200 || len(c.DefaultLocation) > 200 || len(c.DetailHosts) > 5 {
		return fmt.Errorf("career configuration exceeds limits")
	}
	if c.EmptyText != "" && len(strings.TrimSpace(c.EmptyText)) < 8 {
		return fmt.Errorf("empty-list notice must have at least eight characters")
	}
	for _, host := range c.DetailHosts {
		if host == "" || strings.ContainsAny(host, "/:?#@ \t\n") {
			return fmt.Errorf("detail hosts must be hostnames")
		}
	}
	return nil
}

type careerPage struct{ http HTMLGetter }

func NewCompanyCareerPage(c HTMLGetter) Source { return careerPage{http: c} }
func (careerPage) Provider() string            { return "careerpage" }

// Every configured page must terminate its pagination; an error, loop, cap or
// unreadable detail fails the board, so a failed crawl cannot authorize closure.
func (careerPage) fullBoardListing() {}

func careerNodes(root *html.Node, selector string) []*html.Node {
	if selector == "" {
		return nil
	}
	sel, err := cascadia.Parse(selector)
	if err != nil {
		return nil
	}
	return cascadia.QueryAll(root, sel)
}
func careerText(root *html.Node, selector string) string {
	nodes := careerNodes(root, selector)
	if len(nodes) == 0 {
		return ""
	}
	return strings.Join(strings.Fields(textContent(nodes[0])), " ")
}
func careerBody(root *html.Node, selector string) string {
	var b strings.Builder
	for _, n := range careerNodes(root, selector) {
		b.WriteString(innerHTML(n))
		b.WriteString("\n")
	}
	return sanitizeHTML(b.String())
}
func careerVisibleText(root *html.Node) string {
	var b strings.Builder
	walk(root, func(n *html.Node) bool {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "head" || n.Data == "template") {
			return false
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		return true
	})
	return strings.Join(strings.Fields(b.String()), " ")
}
func careerIdentity(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
func careerLink(base *url.URL, raw string, hosts []string) (string, error) {
	ref, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	u := base.ResolveReference(ref)
	canonical, err := CareerPageURL(u.String())
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(u.Hostname(), base.Hostname()) {
		allowed := false
		for _, host := range hosts {
			if strings.EqualFold(host, u.Hostname()) {
				allowed = true
			}
		}
		if !allowed {
			return "", fmt.Errorf("job link leaves configured host: %s", u.Hostname())
		}
	}
	return canonical, nil
}

func (s careerPage) Fetch(ctx context.Context, e CompanyEntry) ([]Job, error) {
	raw, err := CareerPageURL(e.Board)
	if err != nil {
		return nil, err
	}
	cfg := CareerPageConfig{}
	if e.CareerPage != nil {
		cfg = *e.CareerPage
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	base, _ := url.Parse(raw)
	next := raw
	visited := map[string]bool{}
	links := map[string]bool{}
	jobs := []Job{}
	for page := 0; next != ""; page++ {
		if page >= 20 || visited[next] {
			return nil, fmt.Errorf("careerpage: pagination cap or loop")
		}
		visited[next] = true
		root, err := s.http.GetHTML(ctx, next)
		if err != nil {
			return nil, fmt.Errorf("careerpage: listing: %w", err)
		}
		if cfg.Links == "" {
			for _, node := range LDJobPostings(root) {
				job, err := careerLD(node, e, next, cfg.DefaultLocation)
				if err != nil {
					return nil, err
				}
				if !links[job.URL] {
					links[job.URL] = true
					jobs = append(jobs, job)
				}
			}
		}
		selector := cfg.Links
		if selector == "" {
			selector = `a[href*="/job/"],a[href*="/jobs/"]`
		}
		count := len(careerNodes(root, selector))
		for _, n := range careerNodes(root, selector) {
			href := Attr(n, "href")
			if href == "" {
				return nil, fmt.Errorf("careerpage: job-link selector matched a node without href")
			}
			loc, err := careerLink(base, href, cfg.DetailHosts)
			if err != nil {
				return nil, err
			}
			if loc == raw || links[loc] {
				continue
			}
			links[loc] = true
			if len(links) > 200 {
				return nil, fmt.Errorf("careerpage: more than 200 jobs; configure a dedicated adapter")
			}
			detail, err := s.http.GetHTML(ctx, loc)
			if err != nil {
				jobs = append(jobs, Job{ExternalID: careerIdentity(loc), URL: loc, Company: e.Company, Unreadable: true})
				continue
			}
			var job Job
			if cfg.Body == "" {
				nodes := LDJobPostings(detail)
				if len(nodes) != 1 {
					jobs = append(jobs, Job{ExternalID: careerIdentity(loc), URL: loc, Company: e.Company, Unreadable: true})
					continue
				}
				job, err = careerLD(nodes[0], e, loc, cfg.DefaultLocation)
				if err != nil {
					jobs = append(jobs, Job{ExternalID: careerIdentity(loc), URL: loc, Company: e.Company, Unreadable: true})
					continue
				}
			} else {
				titleSelector := cfg.Title
				if titleSelector == "" {
					titleSelector = "h1"
				}
				job = Job{Title: careerText(detail, titleSelector), Description: careerBody(detail, cfg.Body), Company: e.Company, URL: loc, Location: firstNonEmpty(careerText(detail, cfg.Location), cfg.DefaultLocation)}
				if cfg.Posted != "" {
					nodes := careerNodes(detail, cfg.Posted)
					if len(nodes) > 0 {
						job.PostedAt = parseRFC3339OrDate(firstNonEmpty(Attr(nodes[0], "datetime"), textContent(nodes[0])))
					}
				}
				if job.Title == "" || job.Description == "" {
					jobs = append(jobs, Job{ExternalID: careerIdentity(loc), URL: loc, Company: e.Company, Unreadable: true})
					continue
				}
			}
			job.URL = loc
			job.ExternalID = careerIdentity(loc)
			jobs = append(jobs, job)
		}
		if count == 0 && len(LDJobPostings(root)) == 0 && (cfg.EmptyText == "" || !strings.Contains(strings.ToLower(careerVisibleText(root)), strings.ToLower(cfg.EmptyText))) {
			return nil, fmt.Errorf("careerpage: no verified jobs or configured empty-list notice; page may require JavaScript or changed markup")
		}
		next = ""
		if cfg.Next != "" {
			nodes := careerNodes(root, cfg.Next)
			if len(nodes) > 1 {
				return nil, fmt.Errorf("careerpage: pagination selector is ambiguous")
			}
			if len(nodes) == 1 {
				next, err = careerLink(base, Attr(nodes[0], "href"), nil)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if len(jobs) > 200 {
		return nil, fmt.Errorf("careerpage: more than 200 jobs")
	}
	if len(jobs) > 0 {
		read := false
		for _, j := range jobs {
			if !j.Unreadable {
				read = true
			}
		}
		if !read {
			return nil, fmt.Errorf("careerpage: no listed job could be read")
		}
	}
	return jobs, nil
}

func careerLD(raw json.RawMessage, e CompanyEntry, fallback, location string) (Job, error) {
	var d struct {
		sitemapldjsonPosting
		URL          string `json:"url"`
		ValidThrough string `json:"validThrough"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return Job{}, err
	}
	if d.Title == "" || d.Description == "" {
		return Job{}, fmt.Errorf("careerpage: incomplete JobPosting")
	}
	loc := fallback
	if d.URL != "" {
		base, _ := url.Parse(fallback)
		var err error
		loc, err = careerLink(base, d.URL, nil)
		if err != nil {
			return Job{}, err
		}
	}
	removed := false
	if deadline := careerDeadline(d.ValidThrough); deadline != nil {
		removed = deadline.Before(time.Now())
	}
	return Job{ExternalID: careerIdentity(loc), URL: loc, Company: firstNonEmpty(d.HiringOrganization.Name, e.Company), Title: d.Title, Description: sanitizeHTML(d.Description), Location: firstNonEmpty(d.location(), location), Countries: d.countries(), EmploymentType: schemaEmploymentRaw(d.EmploymentType), Remote: strings.EqualFold(d.JobLocationType, "TELECOMMUTE"), PostedAt: parseRFC3339OrDate(d.DatePosted), ApplicationDeadline: careerDeadline(d.ValidThrough), Removed: removed}, nil
}

// Deadlines may be in the future; the posted-at parser deliberately rejects them.
func careerDeadline(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			if layout == "2006-01-02" {
				t = t.AddDate(0, 0, 1)
			}
			return &t
		}
	}
	return nil
}
