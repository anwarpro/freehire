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
	"github.com/google/uuid"
	"github.com/strelov1/freehire/internal/identity/auth"
	"github.com/strelov1/freehire/internal/ingest/teamreview"
	"github.com/strelov1/freehire/internal/platform/db"
)

func TestStudentJobFollowupWorkflow(t *testing.T) {
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
	manager := user("manager@followup.test", "moderator")
	manager2 := user("other@followup.test", "admin")
	reviewer := user("reviewer@followup.test", "user")
	next := user("next@followup.test", "user")
	ordinary := user("ordinary@followup.test", "user")
	exec(`INSERT INTO team_members(user_id,name) VALUES($1,'Reviewer'),($2,'Next reviewer')`, reviewer, next)
	issuer := auth.NewIssuer("student-followup-test", time.Hour)
	cookie := func(id int64) string {
		t.Helper()
		v, err := issuer.Issue(id, testTokenVersion)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	mc, m2c, rc, nc, oc := cookie(manager), cookie(manager2), cookie(reviewer), cookie(next), cookie(ordinary)
	app := fiber.New(fiber.Config{ErrorHandler: RenderError})
	newTeamReviewHandlers(pool, q).register(app.Group("/api/v1"), middleware{cookie: auth.RequireAuth(issuer, testVersions)})
	call := func(method, path, token string, input any) (int, []byte) {
		body, _ := json.Marshal(input)
		req := httptest.NewRequest(method, "/api/v1/team"+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Cookie", auth.CookieName+"="+token)
		}
		res, err := app.Test(req, 10000)
		if err != nil {
			return 0, []byte(err.Error())
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		if token != "" && !strings.Contains(res.Header.Get("Cache-Control"), "no-store") {
			return 0, []byte("missing no-store")
		}
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
	record := func(raw []byte) db.TeamStudentFollowup {
		t.Helper()
		var out struct {
			Data db.TeamStudentFollowup `json:"data"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out.Data
	}
	queue := func(raw []byte) teamreview.StudentFollowupQueue {
		t.Helper()
		var out teamreview.StudentFollowupQueue
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	for _, slug := range []string{"student-job", "other-job", "private-student-job", "closed-student-job"} {
		exec(`INSERT INTO jobs(source,external_id,url,public_slug,title,company,description,is_private) VALUES('student-test',$1,'https://company.test/'||$1,$1,'React developer','Example','React required',$2)`, slug, strings.HasPrefix(slug, "private"))
	}
	exec(`UPDATE jobs SET closed_at=now() WHERE public_slug='closed-student-job'`)
	path := "/jobs/student-job/students"
	want("GET", "/student-followups", "", nil, 401)
	want("GET", "/student-followups", oc, nil, 403)
	want("POST", path, oc, teamreview.StudentFollowupCreate{}, 403)
	want("GET", path, rc, nil, 404)
	create := teamreview.StudentFollowupCreate{StudentRef: "PH-42", StudentName: "Example student", ProfileURL: "https://school.test/students/42", Reason: "Verified React practice and interest in frontend roles"}
	want("POST", "/jobs/private-student-job/students", mc, create, 404)
	want("POST", "/jobs/closed-student-job/students", mc, create, 400)
	want("POST", path, rc, create, 404)
	want("PUT", "/jobs/student-job", mc, teamreview.ReviewInput{AssigneeID: &reviewer, Status: "pending"}, 200)
	f := record(want("POST", path, rc, create, 201))
	if f.Version != 1 || f.Status != "suggested" {
		t.Fatal("invalid initial state")
	}
	detail := fmt.Sprintf("%s/%d", path, f.ID)
	duplicate := create
	duplicate.StudentRef = " ph-42 "
	want("POST", path, mc, duplicate, 409)
	invalid := create
	invalid.StudentRef = "PH-43"
	invalid.ProfileURL = "javascript:alert(1)"
	want("POST", path, rc, invalid, 400)
	invalid.ProfileURL = "https://user:password@school.test/43"
	want("POST", path, rc, invalid, 400)
	want("GET", fmt.Sprintf("/jobs/other-job/students/%d", f.ID), mc, nil, 404)
	want("GET", detail, oc, nil, 403)
	want("GET", detail, nc, nil, 404)
	want("GET", detail+"/history", nc, nil, 404)
	var history struct {
		Data []db.ListStudentFollowupEventsRow `json:"data"`
	}
	json.Unmarshal(want("GET", detail+"/history", rc, nil, 200), &history)
	if len(history.Data) != 1 || history.Data[0].Action != "created" || history.Data[0].Actor != "Reviewer" {
		t.Fatal("creation audit missing")
	}
	update := teamreview.StudentFollowupUpdate{StudentName: f.StudentName, ProfileURL: f.ProfileUrl, Reason: f.Reason, Status: "informed", ExpectedVersion: 1}
	want("PUT", detail, rc, update, 400)
	contact := teamreview.StudentContactInput{Channel: "whatsapp", ContactedAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano), Note: "Manually shared source link", RequestID: uuid.NewString(), ExpectedVersion: 1}
	future := contact
	future.ContactedAt = time.Now().Add(time.Hour).Format(time.RFC3339)
	want("POST", detail+"/contacts", rc, future, 400)
	badChannel := contact
	badChannel.Channel = "auto-send"
	want("POST", detail+"/contacts", rc, badChannel, 400)
	f = record(want("POST", detail+"/contacts", rc, contact, 200))
	if f.Status != "informed" || f.Version != 2 {
		t.Fatal("manual contact did not mark informed")
	}
	replay := record(want("POST", detail+"/contacts", rc, contact, 200))
	if replay.Version != f.Version {
		t.Fatal("contact retry added version")
	}
	changed := contact
	changed.Note = "changed retry"
	want("POST", detail+"/contacts", rc, changed, 409)
	want("POST", detail+"/contacts", mc, contact, 409)
	update.Status = "applied"
	update.Notes = "Student confirmed application"
	update.ExpectedVersion = 2
	date := "2026-01-01"
	update.FollowUp = &date
	f = record(want("PUT", detail, rc, update, 200))
	if f.Version != 3 {
		t.Fatal("update version")
	}
	want("PUT", detail, rc, update, 409)
	update.ExpectedVersion = 3
	f = record(want("PUT", detail, rc, update, 200))
	if f.Version != 3 {
		t.Fatal("unchanged save generated audit")
	}
	contact.RequestID = uuid.NewString()
	contact.ExpectedVersion = 3
	f = record(want("POST", detail+"/contacts", rc, contact, 200))
	if f.Status != "applied" || f.Version != 4 {
		t.Fatal("contact overwrote advanced outcome")
	}
	rows := queue(want("GET", "/student-followups?q=ph-42&status=applied&overdue=true", rc, nil, 200))
	if len(rows.Data) != 1 || !rows.Data[0].Overdue || rows.Data[0].ContactCount != 2 {
		t.Fatal("filters/contact count missing")
	}
	if len(queue(want("GET", "/student-followups?q=not-found", rc, nil, 200)).Data) != 0 {
		t.Fatal("search should be empty")
	}
	want("GET", "/student-followups?status=invalid", mc, nil, 400)
	want("GET", "/student-followups?overdue=wat", mc, nil, 400)
	// Two writers using the same revision cannot silently replace each other's notes.
	update.ExpectedVersion = 4
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i, c := range []string{mc, m2c} {
		wg.Add(1)
		go func(i int, c string) {
			defer wg.Done()
			in := update
			in.Notes = fmt.Sprintf("Manager note %d", i)
			code, _ := call("PUT", detail, c, in)
			codes <- code
		}(i, c)
	}
	wg.Wait()
	close(codes)
	var successes, conflicts int
	for code := range codes {
		if code == 200 {
			successes++
		} else if code == 409 {
			conflicts++
		} else {
			t.Fatalf("concurrent status %d", code)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal("lost update")
	}
	// A duplicate contact submitted concurrently records only one new event.
	contact.ExpectedVersion = 5
	contact.RequestID = uuid.NewString()
	codes = make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); code, _ := call("POST", detail+"/contacts", rc, contact); codes <- code }()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 200 {
			t.Fatalf("contact retry status %d", code)
		}
	}
	var events, version int64
	if err := pool.QueryRow(ctx, `SELECT count(*),max(version) FROM team_student_followup_events WHERE followup_id=$1`, f.ID).Scan(&events, &version); err != nil {
		t.Fatal(err)
	}
	if events != 6 || version != 6 {
		t.Fatalf("events %d version %d", events, version)
	}
	// Pagination and a literal '%' query do not depend on the first page.
	exec(`INSERT INTO team_student_followups(job_id,student_ref,reason) SELECT job_id,'extra-'||n,'Relevant experience' FROM job_reviews CROSS JOIN generate_series(1,52) n WHERE assignee_id=$1`, reviewer)
	first := queue(want("GET", path, rc, nil, 200))
	second := queue(want("GET", path+"?offset=50", rc, nil, 200))
	if len(first.Data) != 50 || !first.Meta.HasMore || len(second.Data) != 3 {
		t.Fatal("pagination")
	}
	if len(queue(want("GET", path+"?q=%25", rc, nil, 200)).Data) != 0 {
		t.Fatal("query unexpectedly treated as wildcard")
	}
	// Reassignment revokes old reviewer's access and transfers the whole job history.
	want("PUT", "/jobs/student-job", mc, teamreview.ReviewInput{AssigneeID: &next, Status: "pending", ExpectedVersion: 1}, 200)
	want("GET", detail, rc, nil, 404)
	want("GET", detail+"/history", rc, nil, 404)
	want("POST", detail+"/contacts", rc, contact, 404)
	if len(queue(want("GET", "/student-followups", rc, nil, 200)).Data) != 0 {
		t.Fatal("former assignee sees student data")
	}
	want("GET", detail, nc, nil, 200)
	want("GET", detail+"/history", nc, nil, 200)
	exec(`UPDATE team_members SET active=false WHERE user_id=$1`, next)
	want("GET", detail, nc, nil, 403)
	want("GET", detail+"/history", nc, nil, 403)
	want("PUT", detail, nc, update, 403)
	// Closure preserves outcomes/history but prevents new links and contact records.
	exec(`UPDATE jobs SET closed_at=now() WHERE public_slug='student-job'`)
	want("POST", path, mc, create, 400)
	contact.ExpectedVersion = 6
	contact.RequestID = uuid.NewString()
	want("POST", detail+"/contacts", mc, contact, 400)
	update.ExpectedVersion = 6
	update.Status = "closed"
	want("PUT", detail, mc, update, 200)
	want("GET", detail+"/history", mc, nil, 200)
	exec(`UPDATE jobs SET is_private=true WHERE public_slug='student-job'`)
	want("GET", detail, mc, nil, 404)
	want("GET", detail+"/history", mc, nil, 404)
	want("PUT", detail, mc, update, 404)
	if len(queue(want("GET", "/student-followups", mc, nil, 200)).Data) != 0 {
		t.Fatal("private posting leaked student data")
	}
}
