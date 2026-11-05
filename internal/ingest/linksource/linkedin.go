package linksource

import (
	"context"
	"fmt"
	"net/url"

	"github.com/strelov1/freehire/internal/ingest/sources"
	"github.com/strelov1/freehire/internal/platform/externalid"
)

type linkedin struct{ http Client }

func NewLinkedIn(c Client) Source      { return linkedin{http: c} }
func (linkedin) Source() string        { return "linkedin" }
func (linkedin) Match(u *url.URL) bool { return u != nil && sources.LinkedInJobID(u.String()) != "" }
func (s linkedin) Resolve(ctx context.Context, raw string) (sources.Job, bool, error) {
	id := sources.LinkedInJobID(raw)
	if id == "" {
		return sources.Job{}, false, nil
	}
	j, err := sources.FetchLinkedInJob(ctx, s.http, id)
	if err != nil {
		return sources.Job{}, false, err
	}
	if j.Removed {
		return sources.Job{}, false, nil
	}
	if j.Unreadable {
		return sources.Job{}, false, fmt.Errorf("linkedin: public job detail is unreadable; open the original posting or use the moderator manual form")
	}
	j.ExternalID = externalid.Namespace("", j.ExternalID)
	return j, true, nil
}
