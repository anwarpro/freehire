//go:build integration

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/strelov1/freehire/internal/identity/auth"
	"github.com/strelov1/freehire/internal/ingest/boardcatalog"
	"github.com/strelov1/freehire/internal/ingest/companycareer"
	"github.com/strelov1/freehire/internal/ingest/sources"
	"github.com/strelov1/freehire/internal/platform/db"
)

type careerFixtureSource struct{}

func (careerFixtureSource) Provider() string { return "careerpage" }
func (careerFixtureSource) Fetch(_ context.Context, e sources.CompanyEntry) ([]sources.Job, error) {
	return []sources.Job{{ExternalID: "1", URL: "https://company.test/job/1", Company: e.Company, Title: "Python Engineer", Description: "Build Python services"}}, nil
}
func TestCompanyCareerManagement(t *testing.T) {
	pool := startPostgres(t)
	ctx := context.Background()
	q := db.New(pool)
	var moderator, member int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,role) VALUES('career-mod@example.test','moderator') RETURNING id`).Scan(&moderator); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users(email) VALUES('career-user@example.test') RETURNING id`).Scan(&member); err != nil {
		t.Fatal(err)
	}
	issuer := auth.NewIssuer("test-career-secret", time.Hour)
	modCookie, _ := issuer.Issue(moderator, testTokenVersion)
	memberCookie, _ := issuer.Issue(member, testTokenVersion)
	repo := boardcatalog.NewQueriesRepository(q)
	h := &companyCareerHandlers{queries: q, service: &companycareer.Service{Repo: repo, Registry: map[string]sources.Source{"careerpage": careerFixtureSource{}}}}
	app := fiber.New(fiber.Config{ErrorHandler: RenderError})
	h.register(app.Group("/api/v1"), middleware{key: auth.RequireAuthOrKey(issuer, testVersions, apiKeys{q}), moderator: auth.RequireRole(q, "moderator"), outboundFetch: func(c *fiber.Ctx) error { return c.Next() }})
	call := func(method, path, cookie string, input any) (int, []byte) {
		t.Helper()
		var body []byte
		if input != nil {
			body, _ = json.Marshal(input)
		}
		r := httptest.NewRequest(method, "/api/v1/moderation/career-pages"+path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if cookie != "" {
			r.Header.Set("Cookie", auth.CookieName+"="+cookie)
		}
		res, err := app.Test(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		return res.StatusCode, raw
	}
	if code, _ := call("GET", "", "", nil); code != 401 {
		t.Fatalf("guest got %d", code)
	}
	if code, _ := call("GET", "", memberCookie, nil); code != 403 {
		t.Fatalf("member got %d", code)
	}
	input := companycareer.Input{Company: "Example Bangladesh", URL: "https://company.test/careers", Config: &sources.CareerPageConfig{Links: "a.job", Body: "article"}}
	if code, raw := call("POST", "/preview", modCookie, input); code != 200 {
		t.Fatalf("preview: %d %s", code, raw)
	}
	code, raw := call("POST", "", modCookie, input)
	if code != 201 {
		t.Fatalf("enable: %d %s", code, raw)
	}
	var result struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	entries, err := boardcatalog.LoadForProvider(ctx, repo, "careerpage")
	if err != nil || len(entries) != 1 || entries[0].CareerPage == nil || entries[0].CareerPage.Body != "article" {
		t.Fatalf("configuration was not persisted: %+v %v", entries, err)
	}
	if code, _ := call("POST", "", modCookie, input); code != 409 {
		t.Fatalf("duplicate returned %d", code)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO board_health(provider,board,region,consecutive_failures,last_error,last_run_at) VALUES('careerpage','https://company.test/careers','',2,'secret-upstream-token',now())`); err != nil {
		t.Fatal(err)
	}
	if code, raw := call("GET", "", modCookie, nil); code != 200 || !strings.Contains(string(raw), "collection_failed") || strings.Contains(string(raw), "secret-upstream-token") {
		t.Fatalf("health/privacy: %d %s", code, raw)
	}
	if code, raw := call("DELETE", "/"+strconv.FormatInt(result.Data.ID, 10), modCookie, nil); code != 204 {
		t.Fatalf("pause: %d %s", code, raw)
	}
	entries, err = boardcatalog.LoadForProvider(ctx, repo, "careerpage")
	if err != nil || len(entries) != 0 {
		t.Fatal("paused source is still scheduled")
	}
}
