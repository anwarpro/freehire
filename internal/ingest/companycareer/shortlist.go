package companycareer

import "github.com/strelov1/freehire/internal/ingest/sources"

type Seed struct {
	Company  string                    `json:"company"`
	URL      string                    `json:"url"`
	Provider string                    `json:"provider,omitempty"`
	Board    string                    `json:"board,omitempty"`
	Config   *sources.CareerPageConfig `json:"config,omitempty"`
	Fallback string                    `json:"fallback,omitempty"`
}

// Shortlist is a curated onboarding list, not the crawl catalogue. Enable verifies
// each source before inserting through boardcatalog; runtime boards stay in Postgres.
func Shortlist() []Seed {
	return []Seed{
		{Company: "Vivasoft Limited", URL: "https://vivasoftltd.com/career/", Provider: "vivasoft", Board: "vivasoftltd.com"},
		{Company: "Cefalo Bangladesh", URL: "https://career.cefalo.com/", Provider: "cefalo", Board: "career.cefalo.com"},
		{Company: "Enosis Solutions", URL: "https://careers.enosisbd.com/", Provider: "pinpoint", Board: "careers.enosisbd.com"},
		{Company: "Dynamic Solution Innovators (DSi)", URL: "https://app.hrythmic.com/recruit/openings/company/dsinnovators/", Provider: "careerpage", Board: "https://app.hrythmic.com/recruit/openings/company/dsinnovators/", Config: &sources.CareerPageConfig{Links: `a[href^="/recruit/openings/"]:not([href*="/company/"])`, Title: "section > div > div.text-2xl", Body: `div.gap-7 > div`, DefaultLocation: "Dhaka, Bangladesh"}},
		{Company: "Ollyo", URL: "https://ollyo.com/careers/roles/", Provider: "careerpage", Board: "https://ollyo.com/careers/roles/", Config: &sources.CareerPageConfig{Links: `a[href^="https://ollyo.com/job/"]`, Title: "h1", Body: `[data-kirki="dpkh6c6o"]`, DefaultLocation: "Dhaka, Bangladesh"}},
		{Company: "weDevs", URL: "https://wedevs.com/career/", Provider: "careerpage", Board: "https://wedevs.com/career/", Config: &sources.CareerPageConfig{Links: `[data-openings-list] a[href^="https://lounge.wedevs.com/job/"]`, Title: "h1", Body: ".erp-recruitment-description", DetailHosts: []string{"lounge.wedevs.com"}}},
		{Company: "Brain Station 23", URL: "https://erp.brainstation-23.com/jobs", Fallback: "Career-site access could not be verified; add individual official job links manually until a supported feed is verified."},
		{Company: "SELISE Bangladesh", URL: "https://selisegroup.com/about-us/", Fallback: "The career page returns a challenge rather than a job listing. Manual official-job intake remains available."},
		{Company: "BJIT", URL: "https://bjitgroup.com/career", Fallback: "The HTML reports no vacancies before JavaScript loads its API. A complete public listing feed has not yet been verified; use manual official-job intake."},
		{Company: "Mediusware", URL: "https://mediusware.com/career", Fallback: "The page loads jobs using JavaScript and its complete public feed has not yet been verified; use manual official-job intake."},
	}
}
