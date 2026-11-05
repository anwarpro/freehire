package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/strelov1/freehire/internal/job/liveness"
)

func TestLinkedInLivenessEvidence(t *testing.T) {
	live := `<h2 class="top-card-layout__title">Engineer</h2><a class="topcard__org-name-link">BD Company</a><span class="topcard__flavor--bullet">Bangladesh</span><div class="description__text"><div class="show-more-less-html__markup"><p>Build software</p></div></div>`
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   liveness.Verdict
	}{
		{"live", 200, live, liveness.Live}, {"footer", 200, live + `<footer>No longer accepting applications</footer>`, liveness.Live},
		{"closed", 200, `<span class="closed-job__flavor">No longer accepting applications</span>`, liveness.Expired},
		{"gone", 404, "", liveness.Expired}, {"removed", 410, "", liveness.Expired},
		{"refusal", 403, "", liveness.Uncertain}, {"throttle", 429, "", liveness.Uncertain}, {"server", 500, "", liveness.Uncertain},
		{"login", 200, `<h1>Sign in</h1>`, liveness.Uncertain}, {"empty", 200, "", liveness.Uncertain},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/123" {
					t.Errorf("wrong identity %s", r.URL.Path)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			got, _ := checkLinkedInLiveAt(context.Background(), srv.Client(), ":123", srv.URL+"/")
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestLinkedInLivenessRefusesRedirectAndInvalidID(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/123" {
			http.Redirect(w, r, "/authwall", http.StatusFound)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()
	got, _ := checkLinkedInLiveAt(context.Background(), srv.Client(), ":123", srv.URL+"/")
	if got != liveness.Uncertain {
		t.Fatal("redirected authwall became a closure")
	}
	calls = 0
	got, _ = checkLinkedInLiveAt(context.Background(), srv.Client(), "123/evil", srv.URL+"/")
	if got != liveness.Uncertain || calls != 0 {
		t.Fatal("invalid ID reached the network")
	}
}
