//go:build integration

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/strelov1/freehire/internal/identity/auth"
	"github.com/strelov1/freehire/internal/ingest/teamreview"
	"github.com/strelov1/freehire/internal/job/curriculum"
	"github.com/strelov1/freehire/internal/platform/db"
)

func TestCurriculumWorkflow(t *testing.T) {
	pool := startPostgres(t)
	ctx := context.Background()
	q := db.New(pool)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	user := func(email, role string) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO users(email,role,email_verified) VALUES($1,$2,true) RETURNING id`, email, role).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	manager := user("manager@curriculum.test", "moderator")
	secondManager := user("second-manager@curriculum.test", "admin")
	reviewer := user("reviewer@curriculum.test", "user")
	nextReviewer := user("next@curriculum.test", "user")
	ordinary := user("ordinary@curriculum.test", "user")
	exec(`INSERT INTO team_members(user_id,name) VALUES($1,'Reviewer'),($2,'Next reviewer')`, reviewer, nextReviewer)
	issuer := auth.NewIssuer("curriculum-test-secret", time.Hour)
	cookie := func(id int64) string {
		t.Helper()
		token, err := issuer.Issue(id, testTokenVersion)
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	managerCookie, secondCookie, reviewerCookie, ordinaryCookie := cookie(manager), cookie(secondManager), cookie(reviewer), cookie(ordinary)
	app := fiber.New(fiber.Config{ErrorHandler: RenderError})
	newTeamReviewHandlers(pool, q).register(app.Group("/api/v1"), middleware{cookie: auth.RequireAuth(issuer, testVersions)})
	call := func(method, path, token string, input any) (int, []byte) {
		body, _ := json.Marshal(input)
		r := httptest.NewRequest(method, "/api/v1/team"+path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Cookie", auth.CookieName+"="+token)
		}
		res, err := app.Test(r, 10000)
		if err != nil {
			return 0, []byte(err.Error())
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		return res.StatusCode, raw
	}
	want := func(method, path, token string, input any, status int) []byte {
		t.Helper()
		code, raw := call(method, path, token, input)
		if code != status {
			t.Fatalf("%s %s = %d want %d: %s", method, path, code, status, raw)
		}
		return raw
	}
	settings := func(raw []byte) teamreview.CurriculumSettings {
		t.Helper()
		var out struct {
			Data teamreview.CurriculumSettings `json:"data"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out.Data
	}
	detail := func(raw []byte) teamreview.CurriculumDetail {
		t.Helper()
		var out struct {
			Data teamreview.CurriculumDetail `json:"data"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out.Data
	}
	queue := func(raw []byte) teamreview.CurriculumQueue {
		t.Helper()
		var out teamreview.CurriculumQueue
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	want("GET", "/curriculum", "", nil, 401)
	want("GET", "/curriculum", ordinaryCookie, nil, 403)
	want("PUT", "/curriculum", ordinaryCookie, teamreview.CurriculumConfigInput{}, 403)
	cfg := settings(want("GET", "/curriculum", managerCookie, nil, 200))
	if len(cfg.Config.Tracks) != 6 || cfg.Version != 1 {
		t.Fatal("approved defaults missing")
	}
	if _, err := curriculum.New(cfg.Config); err != nil {
		t.Fatal(err)
	}
	want("PUT", "/curriculum", reviewerCookie, teamreview.CurriculumConfigInput{ExpectedVersion: 1, Config: cfg.Config}, 403)
	want("GET", "/curriculum/history", reviewerCookie, nil, 403)
	for _, slug := range []string{"curriculum-python", "other-python", "private-python", "closed-python", "unmatched-job"} {
		body := "<h3>Requirements</h3><p>Python and Django required.</p><h3>Preferred qualifications</h3><p>React preferred.</p>"
		if slug == "unmatched-job" {
			body = "Arrange office deliveries."
		}
		exec(`INSERT INTO jobs(source,external_id,url,public_slug,title,company,description,is_private) VALUES('curriculum-test',$1,'https://company.test/'||$1,$1,'Engineer','Example',$2,$3)`, slug, body, slug == "private-python")
	}
	exec(`UPDATE jobs SET closed_at=now() WHERE public_slug='closed-python'`)
	assignment := teamreview.ReviewInput{AssigneeID: &reviewer, Status: "pending"}
	want("PUT", "/jobs/curriculum-python", managerCookie, assignment, 200)
	want("GET", "/curriculum/jobs/private-python", managerCookie, nil, 404)
	want("GET", "/curriculum/jobs/other-python", reviewerCookie, nil, 404)
	// Dirty writes are processed even before the initial seed completes; bounded
	// runs resume until the seed reaches its fixed upper ID.
	for i := 0; i < 10; i++ {
		n, err := curriculum.Drain(ctx, pool, 3)
		if err != nil {
			t.Fatal(err)
		}
		if err = curriculum.RecordRun(pool, n, nil); err != nil {
			t.Fatal(err)
		}
		st, err := q.GetCurriculumScan(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if st.Complete && n == 0 {
			break
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM curriculum_matches m JOIN jobs j ON j.id=m.job_id WHERE j.is_private`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("private projection: %d %v", count, err)
	}
	got := queue(want("GET", "/curriculum/jobs?track=python", reviewerCookie, nil, 200))
	if len(got.Data) != 1 || got.Data[0].PublicSlug != "curriculum-python" {
		t.Fatalf("assignee scope/filter: %+v", got)
	}
	got = queue(want("GET", "/curriculum/jobs?state=closed", managerCookie, nil, 200))
	if len(got.Data) != 1 || got.Data[0].PublicSlug != "closed-python" {
		t.Fatal("closed job projection lost")
	}
	m := detail(want("GET", "/curriculum/jobs/curriculum-python", reviewerCookie, nil, 200))
	if m.Reviewed || !slices.Contains(m.SuggestedTracks, "python") || !slices.Contains(m.SuggestedTracks, "l1") {
		t.Fatalf("suggestions missing: %+v", m)
	}
	if !strings.Contains(string(m.Evidence), `"kind":"required"`) || !strings.Contains(string(m.Evidence), `"kind":"preferred"`) {
		t.Fatal("requirement evidence missing")
	}
	input := func(m teamreview.CurriculumDetail, tracks []string) teamreview.CurriculumReviewInput {
		return teamreview.CurriculumReviewInput{Tracks: tracks, Notes: "Private curriculum reason", ExpectedVersion: m.ReviewVersion, MappingVersion: m.MappingVersion, InputHash: m.InputHash}
	}
	m = detail(want("PUT", "/curriculum/jobs/curriculum-python", reviewerCookie, input(m, []string{}), 200))
	if !m.Reviewed || len(m.EffectiveTracks) != 0 {
		t.Fatal("explicit reviewed no-fit became unreviewed")
	}
	if len(queue(want("GET", "/curriculum/jobs?track=python", reviewerCookie, nil, 200)).Data) != 0 {
		t.Fatal("rejected badge still filtered in")
	}
	m = detail(want("PUT", "/curriculum/jobs/curriculum-python", reviewerCookie, input(m, []string{"l2"}), 200))
	version := m.ReviewVersion
	m = detail(want("PUT", "/curriculum/jobs/curriculum-python", reviewerCookie, input(m, []string{"l2"}), 200))
	if m.ReviewVersion != version {
		t.Fatal("no-op review created audit noise")
	}
	staleInput := input(m, []string{"python"})
	exec(`UPDATE jobs SET description='<p>Python and FastAPI are required. Machine learning preferred.</p>' WHERE public_slug='curriculum-python'`)
	want("PUT", "/curriculum/jobs/curriculum-python", reviewerCookie, staleInput, 409)
	m = detail(want("GET", "/curriculum/jobs/curriculum-python", reviewerCookie, nil, 200))
	if !m.ReviewStale || !slices.Equal(m.EffectiveTracks, []string{"l2"}) {
		t.Fatal("source edit overwrote human labels or hid staleness")
	}
	if len(queue(want("GET", "/curriculum/jobs?review=needs_review", reviewerCookie, nil, 200)).Data) != 1 {
		t.Fatal("stale-review filter failed")
	}
	cfg.Config.Tracks[0].Label = "L1 frontend · updated"
	saved := settings(want("PUT", "/curriculum", managerCookie, teamreview.CurriculumConfigInput{ExpectedVersion: cfg.Version, Config: cfg.Config}, 200))
	if saved.Version != 2 || saved.Progress.Complete {
		t.Fatal("mapping change did not schedule a rebuild")
	}
	want("PUT", "/curriculum", managerCookie, teamreview.CurriculumConfigInput{ExpectedVersion: 1, Config: cfg.Config}, 409)
	want("PUT", "/curriculum/jobs/curriculum-python", reviewerCookie, input(m, []string{"python"}), 409)
	m = detail(want("GET", "/curriculum/jobs/curriculum-python", reviewerCookie, nil, 200))
	if m.MappingVersion != 2 || !m.ReviewStale || !slices.Contains(m.EffectiveTracks, "l2") {
		t.Fatal("mapping refresh erased manual review")
	}
	m = detail(want("PUT", "/curriculum/jobs/curriculum-python", reviewerCookie, input(m, []string{"python"}), 200))
	if m.ReviewStale {
		t.Fatal("reconfirmation remained stale")
	}
	m = detail(want("PUT", "/curriculum/jobs/curriculum-python", reviewerCookie, input(m, nil), 200))
	if m.Reviewed || !slices.Contains(m.EffectiveTracks, "python") {
		t.Fatal("reset did not restore automatic suggestions")
	}
	// Two managers editing the same loaded review cannot silently overwrite it.
	in := input(m, []string{"python"})
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for _, token := range []string{managerCookie, secondCookie} {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			code, _ := call("PUT", "/curriculum/jobs/curriculum-python", token, in)
			codes <- code
		}(token)
	}
	wg.Wait()
	close(codes)
	statuses := []int{}
	for c := range codes {
		statuses = append(statuses, c)
	}
	slices.Sort(statuses)
	if !slices.Equal(statuses, []int{200, 409}) {
		t.Fatalf("lost update: %v", statuses)
	}
	history := want("GET", "/curriculum/jobs/curriculum-python/history", reviewerCookie, nil, 200)
	if !strings.Contains(string(history), "Private curriculum reason") {
		t.Fatal("audit snapshot missing")
	}
	want("GET", "/curriculum/history", managerCookie, nil, 200)
	assignment.AssigneeID = &nextReviewer
	assignment.ExpectedVersion = 1
	want("PUT", "/jobs/curriculum-python", managerCookie, assignment, 200)
	want("GET", "/curriculum/jobs/curriculum-python", reviewerCookie, nil, 404)
	want("GET", "/curriculum/jobs/curriculum-python/history", reviewerCookie, nil, 404)
	if len(queue(want("GET", "/curriculum/jobs", reviewerCookie, nil, 200)).Data) != 0 {
		t.Fatal("former assignee retained curriculum queue")
	}
	exec(`UPDATE team_members SET active=false WHERE user_id=$1`, reviewer)
	want("GET", "/curriculum", reviewerCookie, nil, 403)
	want("GET", "/curriculum/jobs", reviewerCookie, nil, 403)
	exec(`UPDATE jobs SET is_private=true WHERE public_slug='curriculum-python'`)
	if _, err := curriculum.Drain(ctx, pool, 20); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM curriculum_matches m JOIN jobs j ON j.id=m.job_id WHERE j.is_private`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("private transition retained projection: %d %v", count, err)
	}
	want("GET", "/curriculum/jobs/curriculum-python", managerCookie, nil, 404)
}
