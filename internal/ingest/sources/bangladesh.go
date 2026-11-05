package sources

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Vivasoft's career listing links to Novara's public per-job API. The company
// listing is authoritative about which links are open; closed tiles are excluded.
type bangladeshHTTP interface {
	JSONGetter
	HTMLGetter
}
type vivasoft struct{ http bangladeshHTTP }

func NewVivasoft(c bangladeshHTTP) Source { return vivasoft{http: c} }
func (vivasoft) Provider() string         { return "vivasoft" }
func (vivasoft) fullBoardListing()        {}
func (s vivasoft) Fetch(ctx context.Context, e CompanyEntry) ([]Job, error) {
	if e.Board != "vivasoftltd.com" {
		return nil, fmt.Errorf("vivasoft: unsupported board")
	}
	root, err := s.http.GetHTML(ctx, "https://vivasoftltd.com/career/")
	if err != nil {
		return nil, err
	}
	nodes := careerNodes(root, `a[href^="https://app.novara.com.my/jobs/"]:contains("Apply Now")`)
	if len(nodes) == 0 {
		if len(careerNodes(root, `a[href^="https://app.novara.com.my/jobs/"]:contains("Closed")`)) > 0 {
			return []Job{}, nil
		}
		return nil, fmt.Errorf("vivasoft: no verified job links; verify the listing before accepting an empty feed")
	}
	jobs := []Job{}
	seen := map[string]bool{}
	for _, n := range nodes {
		loc := Attr(n, "href")
		if seen[loc] {
			continue
		}
		seen[loc] = true
		u, err := url.Parse(loc)
		if err != nil {
			return nil, err
		}
		slug := strings.TrimPrefix(u.Path, "/jobs/")
		if slug == "" || strings.Contains(slug, "/") {
			return nil, fmt.Errorf("vivasoft: invalid job slug")
		}
		var resp struct {
			Success bool `json:"success"`
			Data    struct {
				ID          string `json:"id"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Status      string `json:"status"`
				Posted      string `json:"posted_at"`
				Deadline    string `json:"deadline"`
				Company     string `json:"company_name"`
				Location    struct {
					City    string `json:"city"`
					Country string `json:"country"`
				} `json:"location"`
			} `json:"data"`
		}
		if err := s.http.GetJSON(ctx, "https://api.app.novara.com.my/api/job-builder/v1/public/jobs/slug/"+url.PathEscape(slug), &resp); err != nil {
			jobs = append(jobs, Job{ExternalID: slug, URL: loc, Company: e.Company, Unreadable: true})
			continue
		}
		d := resp.Data
		if !resp.Success || d.ID == "" || d.Title == "" || d.Description == "" || d.Company != "Vivasoft Limited" {
			jobs = append(jobs, Job{ExternalID: slug, URL: loc, Company: e.Company, Unreadable: true})
			continue
		}
		deadline := careerDeadline(d.Deadline)
		removed := d.Status != "published" || (deadline != nil && !time.Now().Before(*deadline))
		jobs = append(jobs, Job{ExternalID: slug, URL: loc, Title: d.Title, Company: d.Company, Description: sanitizeHTML(d.Description), Location: joinNonEmpty(d.Location.City, d.Location.Country), PostedAt: parseRFC3339OrDate(d.Posted), ApplicationDeadline: deadline, Removed: removed})
	}
	read := false
	for _, j := range jobs {
		if !j.Unreadable {
			read = true
		}
	}
	if len(jobs) > 0 && !read {
		return nil, fmt.Errorf("vivasoft: no listed job could be read")
	}
	return jobs, nil
}

// Cefalo's own API marks expired postings even when its landing page still lists
// them under Current Openings. Honour that flag rather than republishing them.
type cefalo struct{ http bangladeshHTTP }

func NewCefalo(c bangladeshHTTP) Source { return cefalo{http: c} }
func (cefalo) Provider() string         { return "cefalo" }
func (cefalo) fullBoardListing()        {}
func (s cefalo) Fetch(ctx context.Context, e CompanyEntry) ([]Job, error) {
	if e.Board != "career.cefalo.com" {
		return nil, fmt.Errorf("cefalo: unsupported board")
	}
	var rows []struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		Slug     string `json:"slug"`
		Expired  bool   `json:"isExpired"`
		Deadline string `json:"deadline"`
	}
	if err := s.http.GetJSON(ctx, "https://ats.cefalo.com/api/v1/public/jobs", &rows); err != nil {
		return nil, err
	}
	if rows == nil {
		return nil, fmt.Errorf("cefalo: missing job array")
	}
	jobs := []Job{}
	for _, d := range rows {
		if d.ID <= 0 || d.Slug == "" || d.Title == "" || strings.Contains(d.Slug, "/") {
			return nil, fmt.Errorf("cefalo: invalid job identity")
		}
		loc := "https://career.cefalo.com/job/" + url.PathEscape(d.Slug)
		j := Job{ExternalID: fmt.Sprint(d.ID), URL: loc, Title: d.Title, Company: "Cefalo Bangladesh", Location: "Dhaka, Bangladesh", ApplicationDeadline: careerDeadline(d.Deadline), Removed: d.Expired}
		if j.ApplicationDeadline != nil && !time.Now().Before(*j.ApplicationDeadline) {
			j.Removed = true
		}
		if !j.Removed {
			root, err := s.http.GetHTML(ctx, loc)
			if err != nil {
				j.Unreadable = true
				jobs = append(jobs, j)
				continue
			}
			// The role body has a dedicated container, separate from generic benefits.
			j.Description = careerBody(root, `.jobDetails`)
			if j.Description == "" {
				j.Unreadable = true
			}
		}
		jobs = append(jobs, j)
	}
	read := false
	for _, j := range jobs {
		if !j.Unreadable {
			read = true
		}
	}
	if len(jobs) > 0 && !read {
		return nil, fmt.Errorf("cefalo: no listed job could be read")
	}
	return jobs, nil
}
