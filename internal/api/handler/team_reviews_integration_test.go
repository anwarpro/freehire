//go:build integration

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/strelov1/freehire/internal/identity/auth"
	"github.com/strelov1/freehire/internal/ingest/teamreview"
	"github.com/strelov1/freehire/internal/platform/db"
	"golang.org/x/crypto/bcrypt"
)

// Exercise the real cookie boundary, migrations, password policy, and transactional
// assignments together. A retained cookie must never retain revoked team access.
func TestTeamReviewWorkflow(t *testing.T) {
	pool := startPostgres(t)
	ctx := context.Background()
	q := db.New(pool)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	user := func(email, role string, verified bool) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO users(email,role,email_verified) VALUES($1,$2,$3) RETURNING id`, email, role, verified).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	manager := user("manager@team.test", "moderator", true)
	admin := user("admin@team.test", "admin", true)
	ordinary := user("ordinary@team.test", "user", true)
	unverified := user("unverified@team.test", "user", false)
	_ = unverified
	issuer := auth.NewIssuer("team-test-secret", time.Hour)
	cookie := func(id int64) string {
		token, err := issuer.Issue(id, testTokenVersion)
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	app := fiber.New(fiber.Config{ErrorHandler: RenderError})
	newTeamReviewHandlers(pool, q).register(app.Group("/api/v1"), middleware{cookie: auth.RequireAuth(issuer, testVersions)})
	call := func(method, path, token string, input any) (int, []byte) {
		var body []byte
		if input != nil {
			body, _ = json.Marshal(input)
		}
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
	want := func(method, path, token string, input any, expected int) []byte {
		t.Helper()
		code, raw := call(method, path, token, input)
		if code != expected {
			t.Fatalf("%s %s: %d want %d: %s", method, path, code, expected, raw)
		}
		return raw
	}
	modCookie, adminCookie, ordinaryCookie := cookie(manager), cookie(admin), cookie(ordinary)
	want("GET", "/members", "", nil, 401)
	want("GET", "/members", ordinaryCookie, nil, 403)
	want("GET", "/members", adminCookie, nil, 200)
	want("POST", "/members", ordinaryCookie, teamreview.MemberInput{Name: "No", Email: "new@team.test", Password: "password123"}, 403)
	want("POST", "/members", modCookie, teamreview.MemberInput{Name: "Short", Email: "short@team.test", Password: "short"}, 400)
	want("POST", "/members", modCookie, teamreview.MemberInput{Name: "Unverified", Email: "unverified@team.test", UseExisting: true}, 400)
	const initialPassword = "initial-password-123"
	raw := want("POST", "/members", modCookie, teamreview.MemberInput{Name: "Reviewer One", Email: "first@team.test", Password: initialPassword}, 201)
	var created struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatal(err)
	}
	first := created.Data.ID
	u, err := q.GetUserByEmail(ctx, "first@team.test")
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != "user" || u.EmailVerified || !u.PasswordHash.Valid || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash.String), []byte(initialPassword)) != nil {
		t.Fatal("new reviewer must use normal password policy without global privilege or verified email")
	}
	if strings.Contains(string(raw), initialPassword) || strings.Contains(string(raw), u.PasswordHash.String) {
		t.Fatal("credential leaked")
	}
	want("POST", "/members", modCookie, teamreview.MemberInput{Name: "Duplicate", Email: "first@team.test", Password: initialPassword}, 409)
	want("POST", "/members", modCookie, teamreview.MemberInput{Name: "Reviewer Two", Email: "ordinary@team.test", UseExisting: true}, 201)
	want("POST", "/members", modCookie, teamreview.MemberInput{Name: "Duplicate", Email: "ordinary@team.test", UseExisting: true}, 409)
	firstCookie := cookie(first)
	want("GET", "/members", firstCookie, nil, 403)
	want("GET", fmt.Sprintf("/members/%d/history", first), firstCookie, nil, 403)
	access := want("GET", "/me", firstCookie, nil, 200)
	if !strings.Contains(string(access), `"role":"reviewer"`) {
		t.Fatalf("wrong access: %s", access)
	}
	for _, slug := range []string{"public-team-job", "closed-team-job", "private-team-job"} {
		exec(`INSERT INTO jobs(source,external_id,url,title,company,public_slug,is_private) VALUES('team-test',$1,'https://company.test/'||$1,'Python Engineer','Company BD',$1,$2)`, slug, slug == "private-team-job")
	}
	exec(`UPDATE jobs SET closed_at=now() WHERE public_slug='closed-team-job'`)
	input := teamreview.ReviewInput{AssigneeID: &first, Status: "pending", Notes: "Private reviewer notes", ExpectedVersion: 0}
	want("PUT", "/jobs/private-team-job", modCookie, input, 404)
	want("GET", "/jobs/private-team-job", modCookie, nil, 404)
	want("GET", "/jobs/public-team-job", firstCookie, nil, 404)
	want("PUT", "/jobs/public-team-job", firstCookie, input, 404)
	past := "2000-01-01"
	input.FollowUp = &past
	raw = want("PUT", "/jobs/public-team-job", modCookie, input, 200)
	if !strings.Contains(string(raw), `"overdue":true`) || !strings.Contains(string(raw), `"follow_up":"2000-01-01"`) {
		t.Fatalf("calendar date contract: %s", raw)
	}
	want("PUT", "/jobs/closed-team-job", adminCookie, input, 200)
	want("GET", "/jobs/public-team-job", ordinaryCookie, nil, 404)
	want("GET", "/jobs/public-team-job/history", ordinaryCookie, nil, 404)
	want("GET", "/reviews?scope=all", firstCookie, nil, 403)
	want("GET", "/reviews?overdue=garbage", modCookie, nil, 400)
	want("GET", "/reviews?offset=-1", modCookie, nil, 200)
	want("GET", "/reviews?offset=3000000000", modCookie, nil, 400)
	want("GET", "/reviews?status=unknown", modCookie, nil, 400)
	raw = want("GET", "/reviews?scope=mine&overdue=true", firstCookie, nil, 200)
	if !strings.Contains(string(raw), "public-team-job") || !strings.Contains(string(raw), "closed-team-job") || strings.Contains(string(raw), "private-team-job") {
		t.Fatalf("assignment queue: %s", raw)
	}
	input.ExpectedVersion = 1
	input.Status = "in_review"
	input.Notes = "Student follow-up needed"
	want("PUT", "/jobs/public-team-job", firstCookie, input, 200)
	want("PUT", "/jobs/public-team-job", firstCookie, input, 409)
	input.ExpectedVersion = 2
	input.AssigneeID = &ordinary
	want("PUT", "/jobs/public-team-job", firstCookie, input, 403)
	want("PUT", "/jobs/public-team-job", modCookie, input, 200)
	want("GET", "/jobs/public-team-job", firstCookie, nil, 404)
	want("GET", "/jobs/public-team-job/history", firstCookie, nil, 404)
	raw = want("GET", "/jobs/public-team-job/history", ordinaryCookie, nil, 200)
	if !strings.Contains(string(raw), `"action":"reassigned"`) || !strings.Contains(string(raw), "Student follow-up needed") {
		t.Fatalf("missing review audit: %s", raw)
	}
	// Two managers editing the same version produce one winner and one audit event.
	input.ExpectedVersion = 3
	input.Status = "reviewed"
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for _, token := range []string{modCookie, adminCookie} {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			code, _ := call("PUT", "/jobs/public-team-job", token, input)
			codes <- code
		}(token)
	}
	wg.Wait()
	close(codes)
	var successes, conflicts int
	for code := range codes {
		if code == 200 {
			successes++
		}
		if code == 409 {
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent save: successes=%d conflicts=%d", successes, conflicts)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM job_review_events e JOIN jobs j ON j.id=e.job_id WHERE j.public_slug='public-team-job'`).Scan(&count); err != nil || count != 4 {
		t.Fatalf("audit count %d: %v", count, err)
	}
	raw = want("GET", "/reviews?overdue=true", ordinaryCookie, nil, 200)
	if strings.Contains(string(raw), "public-team-job") {
		t.Fatal("completed review must not be overdue")
	}
	memberPath := fmt.Sprintf("/members/%d", ordinary)
	update := teamreview.MemberUpdate{Name: "Reviewer Two", Active: false, ExpectedVersion: 1}
	want("PATCH", memberPath, modCookie, update, 204)
	want("PATCH", memberPath, modCookie, update, 409)
	want("GET", "/reviews", ordinaryCookie, nil, 403)
	want("GET", "/jobs/public-team-job", ordinaryCookie, nil, 403)
	input.ExpectedVersion = 4
	want("PUT", "/jobs/public-team-job", ordinaryCookie, input, 403)
	want("PUT", "/jobs/public-team-job", modCookie, input, 400)
	raw = want("GET", "/jobs/public-team-job", modCookie, nil, 200)
	if !strings.Contains(string(raw), `"assignee_active":false`) {
		t.Fatalf("inactive assignment lost: %s", raw)
	}
	want("GET", memberPath+"/history", modCookie, nil, 200)
	update.Active = true
	update.ExpectedVersion = 2
	want("PATCH", memberPath, adminCookie, update, 204)
	want("GET", "/jobs/public-team-job", ordinaryCookie, nil, 200)
	input.AssigneeID = nil
	input.Status = "not_relevant"
	want("PUT", "/jobs/public-team-job", modCookie, input, 200)
	want("GET", "/jobs/public-team-job", ordinaryCookie, nil, 404)
	raw = want("GET", "/reviews?scope=unassigned", modCookie, nil, 200)
	if !strings.Contains(string(raw), "public-team-job") {
		t.Fatal("unassigned review missing")
	}
	// Invalid edits leave both the current review and its audit trail untouched.
	input.ExpectedVersion = 5
	input.FollowUp = ptrTeam("2026-02-30")
	want("PUT", "/jobs/public-team-job", modCookie, input, 400)
	input.FollowUp = nil
	input.Notes = strings.Repeat("ক", 5001)
	want("PUT", "/jobs/public-team-job", modCookie, input, 400)
	raw = want("GET", "/jobs/public-team-job", modCookie, nil, 200)
	if !strings.Contains(string(raw), `"version":5`) {
		t.Fatal("invalid edit changed version")
	}
	// An API key, even one belonging to a manager, is not a team session.
	r := httptest.NewRequest("GET", "/api/v1/team/members", nil)
	r.Header.Set("Authorization", "Bearer manager-api-key")
	res, err := app.Test(r)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("API key returned %d", res.StatusCode)
	}
}

func ptrTeam(s string) *string { return &s }
