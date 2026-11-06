package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/strelov1/freehire/internal/ai/assistant"
	"github.com/strelov1/freehire/internal/job/curriculum"
	"github.com/strelov1/freehire/internal/job/jobview"
	"github.com/strelov1/freehire/internal/platform/db"
	"github.com/strelov1/freehire/internal/search/search"
)

func (h *assistantHandlers) teamJobRegistry() *assistant.Registry {
	tools := []assistant.Tool{h.facetsTool(), h.teamSearchTool(), h.teamReadJobTool(), h.teamCurriculumTool(), h.teamCourseJobsTool()}
	for i := range tools {
		run := tools[i].Run
		tools[i].Run = func(ctx context.Context, user int64, args json.RawMessage) (any, error) {
			if err := h.teamBotAccess(ctx, user); err != nil {
				return nil, err
			}
			return run(ctx, user, args)
		}
	}
	r := assistant.NewRegistry(tools...)
	r.MaxResultBytes = assistantResultCap
	return r
}
func (h *assistantHandlers) teamCurriculumTool() assistant.Tool {
	return assistant.Tool{Name: "curriculum", Title: "Read current taught-skill rules", Effect: assistant.EffectReads, Description: "Read current course rules and background scan progress. Automatic suggestions describe overlap, not human approval or student eligibility.", Schema: map[string]any{"type": "object", "properties": map[string]any{}}, Run: func(ctx context.Context, user int64, raw json.RawMessage) (any, error) {
		var in struct{}
		if err := assistant.DecodeArgs(raw, &in); err != nil {
			return nil, err
		}
		return h.team.CurriculumSettings(ctx, user)
	}}
}
func (h *assistantHandlers) teamReadJobTool() assistant.Tool {
	t := h.getJobTool()
	t.Description = "Read a public job by its exact slug, including source URL, this site's job link, current closure/freshness and current automatic curriculum evidence. Private postings are never returned."
	t.Run = func(ctx context.Context, user int64, raw json.RawMessage) (any, error) {
		var in struct {
			Slug string `json:"slug"`
		}
		if err := assistant.DecodeArgs(raw, &in); err != nil {
			return nil, err
		}
		row, err := h.publicAssistantJob(ctx, in.Slug)
		if err != nil {
			return nil, err
		}
		view, err := jobview.FromRow(row)
		if err != nil {
			return nil, err
		}
		view.Description = formatDescription(view.Description, "markdown")
		settings, err := h.team.CurriculumSettings(ctx, user)
		if err != nil {
			return nil, err
		}
		engine, err := curriculum.New(settings.Config)
		if err != nil {
			return nil, err
		}
		evidence := teamBotEvidence(engine, row.Title, row.Description)
		return map[string]any{"job": view, "job_link": h.teamJobLink(view.PublicSlug), "source_url": view.URL, "checked_at": time.Now().UTC().Format(time.RFC3339), "mapping_version": settings.Version, "tracks": settings.Config.Tracks, "automatic_curriculum_evidence": evidence, "reviewer_decision": "not accessed by this bot", "source_text_untrusted": true}, nil
	}
	return t
}
func (h *assistantHandlers) teamSearchTool() assistant.Tool {
	t := h.searchJobsTool()
	t.Description = "Search open public jobs using live facets, query and optional posted_within_days. Returned jobs carry original URL, last-seen/closure and a canonical job_link on this installation. This is a bounded page, not exhaustive coverage."
	props := t.Schema["properties"].(map[string]any)
	props["posted_within_days"] = map[string]any{"type": "integer", "description": "Optional recency by posting date, 1–365 days; omitted means any date."}
	t.Run = func(ctx context.Context, _ int64, raw json.RawMessage) (any, error) {
		var in struct {
			Query   string              `json:"query"`
			Filters assistantFilterArgs `json:"filters"`
			Limit   int                 `json:"limit"`
			Offset  int                 `json:"offset"`
			Days    *int                `json:"posted_within_days"`
		}
		if err := assistant.DecodeArgs(raw, &in); err != nil {
			return nil, err
		}
		if in.Offset < 0 || in.Offset > 100000 || len(in.Query) > 1000 {
			return nil, fmt.Errorf("invalid search query or offset")
		}
		if in.Days != nil && (*in.Days < 1 || *in.Days > 365) {
			return nil, fmt.Errorf("posted_within_days must be 1–365")
		}
		vals, err := in.Filters.values()
		if err != nil {
			return nil, err
		}
		if in.Days != nil {
			vals.Set("posted_within_days", fmt.Sprint(*in.Days))
		}
		if h.search == nil || h.search.search == nil {
			return nil, fmt.Errorf("search is unavailable")
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 10
		}
		limit = min(limit, 20)
		var order []string
		if strings.TrimSpace(in.Query) == "" {
			order = []string{"posted_at:desc"}
		}
		result, err := h.search.search.Search(ctx, search.SearchParams{Query: in.Query, Filter: search.FilterFromValues(vals), Sort: order, Limit: limit, Offset: in.Offset})
		if err != nil {
			return nil, err
		}
		if err = h.search.hydrateDescriptions(ctx, result.Hits); err != nil {
			return nil, err
		}
		out := make([]map[string]any, 0, len(result.Hits))
		for _, hit := range result.Hits {
			// Re-read authoritative privacy/closure; the search index is eventually consistent.
			row, err := h.queries.GetJobBySlug(ctx, hit.PublicSlug)
			if err != nil {
				return nil, err
			}
			if row.IsPrivate || row.ClosedAt.Valid {
				continue
			}
			view, err := jobview.FromRow(row)
			if err != nil {
				return nil, err
			}
			if view.ClosedAt != nil {
				continue
			}
			view.Description = formatDescription(view.Description, "markdown")
			out = append(out, map[string]any{"job": view, "job_link": h.teamJobLink(view.PublicSlug), "source_url": view.URL, "source_text_untrusted": true})
		}
		return map[string]any{"jobs": out, "limit": limit, "offset": in.Offset, "total": result.Total, "total_note": "Search-index count; some stale or private entries may be withheld."}, nil
	}
	return t
}
func (h *assistantHandlers) teamCourseJobsTool() assistant.Tool {
	return assistant.Tool{Name: "course_jobs", Title: "Find automatic course overlaps", Effect: assistant.EffectReads, Description: "Find persisted automatic course suggestions for public jobs. Call curriculum for track IDs and scan progress. Results can be incomplete while scanning or queued changes exist; read get_job for fresh evidence. Does not read reviewer labels or notes.", Schema: map[string]any{"type": "object", "properties": map[string]any{"track": map[string]any{"type": "string"}, "source": map[string]any{"type": "string"}, "state": map[string]any{"type": "string", "enum": []string{"active", "closed", "all"}}, "offset": map[string]any{"type": "integer"}}}, Run: func(ctx context.Context, user int64, raw json.RawMessage) (any, error) {
		var in struct {
			Track  string `json:"track"`
			Source string `json:"source"`
			State  string `json:"state"`
			Offset int32  `json:"offset"`
		}
		if err := assistant.DecodeArgs(raw, &in); err != nil {
			return nil, err
		}
		if in.State == "" {
			in.State = "active"
		}
		if in.Offset < 0 || in.Offset > 100000 || (in.State != "active" && in.State != "closed" && in.State != "all") || len(in.Source) > 100 {
			return nil, fmt.Errorf("invalid course search")
		}
		settings, err := h.team.CurriculumSettings(ctx, user)
		if err != nil {
			return nil, err
		}
		found := in.Track == ""
		for _, t := range settings.Config.Tracks {
			found = found || (t.ID == in.Track && t.Enabled)
		}
		if !found {
			return nil, fmt.Errorf("choose an enabled track from curriculum")
		}
		rows, err := h.queries.ListTeamBotCourseJobs(ctx, db.ListTeamBotCourseJobsParams{MappingVersion: settings.Version, Track: in.Track, Source: in.Source, State: in.State, PageOffset: in.Offset})
		if err != nil {
			return nil, err
		}
		more := len(rows) > 10
		if more {
			rows = rows[:10]
		}
		out := make([]map[string]any, 0, len(rows))
		for _, r := range rows {
			out = append(out, map[string]any{"posting": r, "job_link": h.teamJobLink(r.PublicSlug), "source_url": r.URL, "source_text_untrusted": true})
		}
		return map[string]any{"jobs": out, "has_more": more, "offset": in.Offset, "limit": 10, "mapping_version": settings.Version, "scan": settings.Progress, "decision_type": "automatic, unreviewed overlap; human labels are not accessed"}, nil
	}}
}

// Both assistant modes enforce the public-only boundary at their shared row read.
func (h *assistantHandlers) publicAssistantJob(ctx context.Context, slug string) (db.Job, error) {
	row, err := h.queries.GetJobBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) || row.IsPrivate {
		return db.Job{}, fmt.Errorf("no public vacancy with slug %q", slug)
	}
	if err != nil {
		return db.Job{}, fmt.Errorf("job lookup is unavailable")
	}
	return row, nil
}

// Match the original source rather than presentation Markdown: heading syntax must
// not turn a Requirements section into an unqualified mention. Bound source text.
func teamBotEvidence(engine *curriculum.Engine, title, description string) curriculum.Result {
	truncated := utf8.RuneCountInString(description) > 50000
	if truncated {
		description = string([]rune(description)[:50000])
	}
	result := engine.Match(title, description)
	result.Truncated = truncated
	return result
}
