package sources

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/strelov1/freehire/internal/platform/htmltext"
)

// BDJobs' public web client uses category=8 for IT/Telecommunication. This is
// deliberately one catalogue identity: overlapping category boards would namespace
// the same native job ID differently. All listing pages are verified before details
// are fetched, and every refresh re-reads details to observe edits and deadlines.
type bdjobs struct{ http JSONGetter }

const bdjobsAPI = "https://api.bdjobs.com/jobs/api/v1/"
const bdjobsMaxPages = 100

func NewBDJobs(c JSONGetter) Source { return bdjobs{http: c} }
func (bdjobs) Provider() string     { return "bdjobs" }
func (bdjobs) boardless()           {}
func (bdjobs) aggregator()          {}
func (bdjobs) fullBoardListing()    {}
func (bdjobs) fullCatalog()         {}

type bdjobsListing struct {
	ID       string `json:"Jobid"`
	Title    string `json:"jobTitle"`
	Company  string `json:"companyName"`
	Posted   string `json:"publishDate"`
	Deadline string `json:"deadlineDB"`
}

type bdjobsListResponse struct {
	Status  string          `json:"statuscode"`
	Data    []bdjobsListing `json:"data"`
	Premium []bdjobsListing `json:"premiumData"`
	Common  *struct {
		Total *int `json:"total_records_found"`
		Pages *int `json:"totalpages"`
	} `json:"common"`
}

type bdjobsDetail struct {
	ID           string `json:"JobId"`
	Found        string `json:"JobFound"`
	Error        string `json:"error"`
	Company      string `json:"CompnayName"` // The public API spells it this way.
	Title        string `json:"JobTitle"`
	Deadline     string `json:"DeadlineDB"`
	Description  string `json:"JobDescription"`
	Context      string `json:"Context"`
	Requirements string `json:"AdditionJobRequirements"`
	Education    string `json:"EducationRequirements"`
	Benefits     string `json:"OtherBenefits"`
	Location     string `json:"JobLocation"`
	Nature       string `json:"JobNature"`
	Workplace    string `json:"JobWorkPlace"`
	Closed       int    `json:"Closed"`
}

func (s bdjobs) Fetch(ctx context.Context, _ CompanyEntry) ([]Job, error) {
	listed := make([]bdjobsListing, 0)
	seen, pageSets := map[string]bool{}, map[string]bool{}
	positions := map[string]int{}
	total, pages := -1, -1
	for page := 1; ; page++ {
		var resp bdjobsListResponse
		u := fmt.Sprintf(bdjobsAPI+"JobSearch/GetJobSearch?category=8&pg=%d&rpp=50&ToggleJobs=true", page)
		if err := s.http.GetJSON(ctx, u, &resp); err != nil {
			return nil, fmt.Errorf("bdjobs: listing page %d: %w", page, err)
		}
		if resp.Status != "1" || resp.Data == nil || resp.Common == nil || resp.Common.Total == nil || resp.Common.Pages == nil {
			return nil, fmt.Errorf("bdjobs: page %d has no verified listing metadata", page)
		}
		n, p := *resp.Common.Total, *resp.Common.Pages
		if n < 0 || p < 0 || p > bdjobsMaxPages || (n > 0 && (p == 0 || p != (n+49)/50)) || (n == 0 && p > 1) {
			return nil, fmt.Errorf("bdjobs: invalid or oversized listing (%d jobs, %d pages)", n, p)
		}
		if page == 1 {
			total, pages = n, p
		} else if total != n || pages != p {
			return nil, fmt.Errorf("bdjobs: listing changed during pagination (%d/%d jobs, %d/%d pages); retry the complete crawl", total, n, pages, p)
		}
		expected := 50
		if page >= pages {
			expected = total - (page-1)*50
		}
		if len(resp.Data) != expected {
			return nil, fmt.Errorf("bdjobs: incomplete page %d (%d of %d rows)", page, len(resp.Data), expected)
		}
		pageIDs := make([]string, 0, len(resp.Data))
		for _, row := range resp.Data {
			if !bdjobsID(row.ID) || strings.TrimSpace(row.Title) == "" || strings.TrimSpace(row.Company) == "" {
				return nil, fmt.Errorf("bdjobs: malformed listing row on page %d", page)
			}
			pageIDs = append(pageIDs, row.ID)
		}
		slices.Sort(pageIDs)
		pageKey := strings.Join(pageIDs, ",")
		if pageKey != "" && pageSets[pageKey] {
			return nil, fmt.Errorf("bdjobs: repeated listing page %d", page)
		}
		pageSets[pageKey] = true
		// Promoted jobs repeat across pages and can also occur in the regular list.
		for group, rows := range [][]bdjobsListing{resp.Data, resp.Premium} {
			for _, row := range rows {
				if !bdjobsID(row.ID) || strings.TrimSpace(row.Title) == "" || strings.TrimSpace(row.Company) == "" {
					return nil, fmt.Errorf("bdjobs: malformed promoted/listing row")
				}
				if !seen[row.ID] {
					seen[row.ID] = true
					positions[row.ID] = len(listed)
					listed = append(listed, row)
				} else if group == 0 {
					listed[positions[row.ID]] = row
				}
			}
		}
		if page >= pages {
			break
		}
	}
	jobs := make([]Job, 0, len(listed))
	read := 0
	for _, row := range listed {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		j := Job{ExternalID: row.ID, URL: "https://bdjobs.com/h/details/" + row.ID, Title: row.Title, Company: row.Company, PostedAt: parseRFC3339(row.Posted), ApplicationDeadline: bdjobsDeadline(row.Deadline), IsTechHint: true}
		// Details can extend a deadline while a listing/promoted copy is stale.
		j = s.detail(ctx, j)
		if !j.Unreadable {
			read++
		}
		jobs = append(jobs, j)
	}
	if len(listed) > 0 && read == 0 {
		return nil, fmt.Errorf("bdjobs: no listed job detail could be read")
	}
	return jobs, nil
}

func (s bdjobs) detail(ctx context.Context, j Job) Job {
	var resp struct {
		Status  string         `json:"statuscode"`
		Message string         `json:"message"`
		Data    []bdjobsDetail `json:"data"`
	}
	u := bdjobsAPI + "JobSubsystem/jobDetails?jobId=" + j.ExternalID + "&ln=1&IsCorporate=false"
	if err := s.http.GetJSON(ctx, u, &resp); err != nil {
		j.Unreadable = true
		return j
	}
	// This is the API's explicit gone answer, not an empty/malformed success.
	if resp.Status == "1" && strings.EqualFold(resp.Message, "data not found") && len(resp.Data) == 0 {
		j.Removed = true
		return j
	}
	if resp.Status != "0" || len(resp.Data) != 1 || resp.Data[0].ID != j.ExternalID || resp.Data[0].Found != "True" || resp.Data[0].Error != "0" {
		j.Unreadable = true
		return j
	}
	d := resp.Data[0]
	if deadline := bdjobsDeadline(d.Deadline); deadline != nil {
		j.ApplicationDeadline = deadline
	}
	if d.Closed != 0 || (j.ApplicationDeadline != nil && !time.Now().Before(*j.ApplicationDeadline)) {
		j.Removed = true
		return j
	}
	j.Title, j.Company = strings.TrimSpace(d.Title), strings.TrimSpace(d.Company)
	j.Location = strings.TrimSpace(d.Location)
	var body strings.Builder
	for _, section := range [][2]string{{"Job context", d.Context}, {"Responsibilities", d.Description}, {"Requirements", d.Requirements}, {"Education", d.Education}, {"Benefits", d.Benefits}} {
		if strings.TrimSpace(htmltext.ToText(sanitizeHTML(section[1]))) != "" {
			fmt.Fprintf(&body, "<h2>%s</h2>%s", section[0], section[1])
		}
	}
	j.Description = sanitizeHTML(body.String())
	roleText := htmltext.ToText(sanitizeHTML(d.Description + d.Context + d.Requirements))
	if j.Title == "" || j.Company == "" || strings.TrimSpace(roleText) == "" {
		j.Unreadable = true // Includes image-only and inaccessible early-access adverts.
		return j
	}
	switch d.Nature {
	case "Full Time":
		j.EmploymentType = "full_time"
	case "Part Time":
		j.EmploymentType = "part_time"
	case "Internship":
		j.EmploymentType = "internship"
	case "Contractual":
		j.EmploymentType = "contract"
	}
	switch d.Workplace {
	case "Work at office":
		j.WorkMode = "onsite"
	case "Work from home":
		j.Remote, j.WorkMode = true, "remote"
	case "Hybrid":
		j.WorkMode = "hybrid"
	}
	return j
}

func bdjobsID(id string) bool {
	n, err := strconv.ParseUint(id, 10, 64)
	return err == nil && n > 0 && strconv.FormatUint(n, 10) == id
}

// Despite the listing's Z suffix, deadlineDB is a calendar date, not a UTC
// instant. BDJobs displays it as that date. Expiry is the next Dhaka midnight.
func bdjobsDeadline(raw string) *time.Time {
	if len(raw) != 10 && len(raw) != 20 {
		return nil
	}
	if len(raw) == 20 && raw[10:] != "T00:00:00Z" {
		return nil
	}
	d, err := time.ParseInLocation("2006-01-02", raw[:10], time.FixedZone("Asia/Dhaka", 6*60*60))
	if err != nil {
		return nil
	}
	end := d.AddDate(0, 0, 1).UTC()
	return &end
}
