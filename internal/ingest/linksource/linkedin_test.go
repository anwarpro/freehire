package linksource

import (
	"context"
	"testing"
)

const linkedInDetailFixture = `<h2 class="top-card-layout__title">Python Engineer</h2><a class="topcard__org-name-link">Example BD</a><span class="topcard__flavor--bullet">Dhaka, Bangladesh</span><div class="description__text"><div class="show-more-less-html__markup"><p>Build Python services.</p></div></div>`

func TestLinkedInLinkMatchesCollectorIdentity(t *testing.T) {
	client := (&fakeClient{}).route("jobs-guest/jobs/api/jobPosting/123", linkedInDetailFixture, "")
	for _, raw := range []string{"https://bd.linkedin.com/jobs/view/python-engineer-123?trk=shared", "https://www.linkedin.com/jobs/view/123/"} {
		job, ok, err := NewLinkedIn(client).Resolve(context.Background(), raw)
		if err != nil || !ok || job.ExternalID != ":123" || job.URL != "https://www.linkedin.com/jobs/view/123/" {
			t.Fatalf("link identity mismatch: %+v %v %v", job, ok, err)
		}
	}
}

func TestLinkedInLinkDoesNotImportClosedOrUnreadablePages(t *testing.T) {
	for _, body := range []string{`<span class="closed-job__flavor">No longer accepting applications</span>`, `<h1>Sign in to continue</h1>`} {
		client := (&fakeClient{}).route("jobPosting/123", body, "")
		_, ok, err := NewLinkedIn(client).Resolve(context.Background(), "https://www.linkedin.com/jobs/view/123/")
		if ok {
			t.Fatal("invalid vacancy was imported")
		}
		if body == `<h1>Sign in to continue</h1>` && err == nil {
			t.Fatal("blocked page must report failure")
		}
	}
}
