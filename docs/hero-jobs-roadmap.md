# PH Jobs: implementation baseline

Reviewed locally on 2026-11-05. This is a code audit, not a live deployment verification.
Each independently usable feature should have its own PR, based on the internal
deployment branch until the fork establishes another integration branch.

## Existing capabilities and next steps

| Capability | Finding | Evidence | Next step |
| --- | --- | --- | --- |
| Manual job intake | Reusable backend and public submission form; direct moderator publishing, a job-page editor, posting-date controls, and application deadlines use the existing endpoints | [Moderation API](../internal/api/handler/jobs_moderation.go), [submission form](../web/src/lib/components/SubmitView.svelte), [job editor](../web/src/lib/components/ManualJobEditor.svelte) | Verify deadline migrations, expiry scheduling, and staff workflow after deployment |
| Browsing and filtering | Reusable list, search, pagination, job detail, and source picker under All filters; a direct source shortcut is being added | [Jobs view](../web/src/lib/components/JobsView.svelte), [jobs route](../web/src/routes/jobs/+page.server.ts) | Verify the Bangladeshi source and curriculum workflow against actual stored jobs |
| Source ingestion | Reusable adapters, catalog registration, refresh, and indexing operations | [Source integration guide](../ops/hero-jobs/README.md), [source registry](../internal/ingest/sources/registry.go) | Verify selected Bangladeshi career pages and the feasible access method for BDJobs and LinkedIn before implementing collectors |
| Accounts and roles | Reusable authentication and moderator authorization; school-team account management is additional scope | [Role middleware](../internal/identity/auth/middleware.go), [account services](../internal/identity/accounts/AGENTS.md) | Add a complete team account lifecycle, then a reviewer assignment feature |
| Matching | Reusable candidate/job matching; school curriculum badges and school student links are additional scope | [Matching service](../internal/candidate/cvmatch/AGENTS.md), [job view](../internal/job/jobview/AGENTS.md) | Confirm course names/skills, then implement manually reviewed curriculum labels before automatic suggestions |
| Bot | Reusable assistant runtime and job tools; school-specific permissions and deployment configuration need work | [Assistant](../internal/ai/assistant/AGENTS.md), [deployment prerequisites](../ops/hero-jobs/README.md) | Validate the job-discussion use case and model access before exposing student data |

## Feature: moderators publish a curated job

Open `/moderation?tab=publish` with a moderator account, enter the job URL,
title, company and any available details, check Preview, and choose **Publish job**.
The confirmation links to the stored job. Search visibility follows the existing
indexing pipeline. No new migration or collector is required.

The public `/submit` page continues to send jobs to the review queue. Publishing
requires the server's existing moderator authorization, even if a caller bypasses
the UI. A failed request keeps the draft for retry. Known existing URLs are linked
rather than offered for publication again.

The HTTP publishing endpoint already has role/authentication, validation,
idempotency and persistence coverage in
[its integration tests](../internal/api/handler/jobs_moderation_integration_test.go).
This feature adds client endpoint and component tests for publication, permission
visibility, duplicate discovery, in-flight submission, retry, and the public review
workflow. Live end-to-end verification is still required before deployment.

Manual editing, posting/deadline controls, curriculum badges, reviewer assignment,
student follow-ups, source collectors, and bot changes are separate PRs.

## Feature: moderators edit a manually added job

A moderator opening a manually added public job can choose **Edit job** beside
the title. The editor fetches the latest posting, then allows corrections to the
title, company, location, remote flag, and description. URL and source identity
stay fixed. The description uses the shared Markdown editor with a sanitized
preview. Saving sends only changed fields to the existing PATCH endpoint, so an
untouched HTML description does not pass through a Markdown conversion.

The page uses the returned job immediately after a successful save. Failed saves
retain the draft for retry; cancellation discards it without a request. While
saving, controls and dismissal are disabled. A response arriving after navigation
does not replace another job's content. The editor is loaded only on demand and
is not offered to visitors, ordinary members, or crawler-owned jobs. The server's
existing authorization and private-job checks remain authoritative.

The existing backend recalculates search tags from the edited text and location,
queues indexing, and keeps closed jobs closed. The form explains those effects.
This PR changes no database schema or Go behavior. Posting/deadline controls and
structured skill/facet editing remain separate features.

## Feature: set and correct posting dates

The publishing/submission form accepts an optional **Posting date**. The date
picker uses Bangladesh time (UTC+06:00), and the selected day is sent as midnight
in that offset through the existing `posted_at` field. Blank omits the field;
publication resets the selection only after success. Invalid or future dates
are refused before submission.

The moderator editor shows the job's current displayed posting date and provides
an optional **New posting date**. Selecting a different timestamp allows a
date-only correction. Leaving the replacement blank preserves the stored value;
clearing a draft selection undoes that replacement. Failed saves retain the date.
The existing API does not support clearing a stored date with null, so this UI
offers set/correct operations. An equivalent timestamp is treated as unchanged.

The public API already serves an effective posting date: the source timestamp
when usable, otherwise the job's creation time. The editor labels this as the
current *displayed* date and never copies that fallback into an update unless a
replacement is explicitly selected. Existing job-page date display and search
indexing consume the corrected timestamp.

This feature has no Go/schema changes. Application deadlines are tracked
separately: the current job model has no deadline field, and supporting one
requires storage, editing/display, and expiry handling together.

## Feature: application deadlines

Moderators can set **Application deadline** when publishing and replace or clear
it in **Edit job**. Blank means unknown. The date applies through the end of that
calendar day in Bangladesh; the API stores the exclusive next midnight as an
RFC3339 `application_deadline`. Historical deadlines are allowed for accurate
records. The public review queue does not offer this moderator-only control.

`PATCH /api/v1/jobs/:slug` preserves an omitted deadline, replaces a timestamp,
and clears an explicit JSON `null`. An unrelated edit preserves the exact
stored timestamp. Saving an elapsed deadline closes the job immediately. Changing
or clearing an elapsed deadline preserves closure even before the scheduled
sweep, following the existing editing lifecycle. A deliberate same-URL
re-publication retains the existing reopen behavior.

Job details show the last application day and withhold application controls when
expired, including on a page left open (checked every 30 seconds). Fresh API
projections expose an elapsed deadline as `closed_at` immediately. Direct-apply
and new auto-apply requests also check the deadline before proceeding. Queued
auto-apply attempts also check the deadline before invoking the submission driver
and park when expired. SEO uses the
stated deadline rather than estimating a rolling expiry for these jobs.

The existing `search-drain` worker closes due rows before processing removals;
the close and search removal enqueue are one SQL transaction and repeated runs
are idempotent. PH Jobs schedules it every two minutes. Active lists and indexed
results converge on that sweep; an unavailable scheduler/search service can
delay removal, while detail projection and new-application guards still apply.
This does not parse deadlines automatically from collected job descriptions.

Apply migrations 0207–0209 **before** deploying the new API and search worker,
using the [full deployment procedure](../ops/hero-jobs/README.md). They add a
nullable column, widen the closure-reason constraint, and build a partial index
concurrently. Rebuild the app and worker-tools images as well as the frontend.
No search-settings change or catalogue-wide reindex is required. On rollback,
keep the expansive schema; older code tolerates it, but no longer honors deadlines.
The concurrent-index migration documents recovery for an interrupted index build.

Coverage includes calendar boundaries, publication/editing/clearing, exact-time
preservation, immediate projection, API body presence, real-Postgres persistence,
idempotent closure/removal enqueue, and refusal to reopen through an edit.


## Feature: source filtering above the job list

The source filter already exists under **All filters → Source**. The new
**Source** button beside the list's freshness and sort controls opens that
section directly on desktop and mobile. Its badge counts included and excluded
source choices; its accessible label and tooltip describe them by name.

This reuses the existing picker, live source distribution, staged Apply/Cancel,
URL serialization, saved filters, pagination reset, and source summary chips.
Opening the shortcut repeatedly returns to Source even after visiting another
section. Opening All filters normally retains the last visited section. A scope
that fixes or hides the source facet also hides the shortcut.

The picker offers indexed sources, with already selected values retained for
removal even when their current count is zero. Display labels for manually
recorded `bdjobs` and `linkedin` sources use BDJobs and LinkedIn. These labels do
not install collectors or claim that either platform has been connected.

Validation includes the requested-section reopening behavior, fallback for a
section hidden by scope, accessible source descriptions, and selection updates.
The live search API was verified to return only Greenhouse jobs for
`source=greenhouse`. This feature changes only frontend code/documentation;
no schema, API, worker, search-settings update, or reindex is needed.

Q05 remains open for the active/closed browsing workflow, to be delivered as its
own independent feature.


## Feature: browse active and closed jobs

The standalone job feed now links **Active jobs** and **Closed / expired jobs**.
The archive at `/jobs/closed` reads retained public canonical postings from
PostgreSQL; it does not put expired vacancies back into the active search index.
Private pasted descriptions and duplicate repost rows are excluded.

The archive supports source, skill-tag and country-code includes/exclusions,
plus Any/All skill matching. Controls use a normal GET form, so filters, reloads,
and page links work without JavaScript. Switching status carries only these
shared filters and resets paging. The page explains that other active-search
filters are not carried over; the archive API returns 400 for unsupported filters
rather than pretending to apply them. It does not provide active search's full
keyword, geography, matching, or salary vocabulary.

Jobs are ordered by closure time and public slug. **Older closed jobs** uses an
opaque cursor over those public fields, retaining timestamp precision; **Newest
closed jobs** resets it. The endpoint fetches one extra row to detect another
page and never counts the entire retained catalogue. The shared public-read
budget and a five-second database deadline bound requests. Existing job cards
show the closed state and withhold Apply controls; the archive adds an explicit
validated HTTP(S) original-posting link. Detail links keep their existing behavior.

Apply migrations 0210–0211 before deploying the new API. They build the public
archive ordering index and the skill/country GIN index concurrently; their
headers explain recovery after an interrupted build. Use the full app/web/tools
[deployment procedure](../ops/hero-jobs/README.md). No Meilisearch settings change
or reindex is needed. Existing closed jobs require no data backfill; deadline
expiry joins the archive after the existing scheduled close persists it.

The [API reference](API.md) documents `/jobs/closed`. Coverage includes private,
active and duplicate exclusion, combined filters, Any/All skills, empty results,
original links, equal-timestamp pagination without duplicate rows, cursor
validation/precision, and status navigation that clears stale pagination.
Built-app SSR smoke checks also verified populated and empty archives, original
links, withheld Apply controls, cursor navigation, and 400/503 error handling.
