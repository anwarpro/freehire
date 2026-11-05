package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseClosedJobs(t *testing.T) {
	p, err := parseClosedJobs(url.Values{"source": {" Greenhouse , BDJobs", "greenhouse"}, "skills": {"Python, React"}, "countries": {"BD"}, "skills_mode": {"and"}, "source_exclude": {"linkedin"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Sources, []string{"greenhouse", "bdjobs"}) || !reflect.DeepEqual(p.Skills, []string{"python", "react"}) || !p.AllSkills || p.Countries[0] != "bd" || p.ExcludedSources[0] != "linkedin" {
		t.Fatalf("unexpected parsed filters: %+v", p)
	}
}

func TestClosedJobsRejectsMisleadingOrUnboundedInput(t *testing.T) {
	for _, values := range []url.Values{
		{"q": {"python"}}, {"offset": {"1000000"}}, {"before": {"not-a-cursor"}},
		{"skills_mode": {"invalid"}}, {"countries": {"Bangladesh"}},
		{"source": {strings.Repeat("a", 1025)}},
	} {
		if _, err := parseClosedJobs(values); err == nil {
			t.Fatalf("accepted invalid input: %v", values)
		}
	}
}

func TestClosedCursorPreservesTimestampPrecision(t *testing.T) {
	at := time.Date(2026, 11, 5, 12, 0, 0, 123456000, time.UTC)
	b, err := json.Marshal(closedJobCursor{ClosedAt: at, Slug: "public-slug"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := parseClosedJobs(url.Values{"before": {base64.RawURLEncoding.EncodeToString(b)}})
	if err != nil || !p.BeforeClosedAt.Valid || !p.BeforeClosedAt.Time.Equal(at) || p.BeforeSlug != "public-slug" {
		t.Fatalf("cursor lost precision: %+v %v", p, err)
	}
}
