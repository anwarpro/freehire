# PH Jobs: implementation baseline

Reviewed locally on 2026-11-05. This is a code audit, not a live deployment verification.
Each independently usable feature should have its own PR, based on the internal
deployment branch until the fork establishes another integration branch.

## Existing capabilities and next steps

| Capability | Finding | Evidence | Next step |
| --- | --- | --- | --- |
| Manual job intake | Reusable backend and public submission form; direct moderator publishing, a job-page editor, and posting-date controls use the existing endpoints | [Moderation API](../internal/api/handler/jobs_moderation.go), [submission form](../web/src/lib/components/SubmitView.svelte), [job editor](../web/src/lib/components/ManualJobEditor.svelte) | Verify each feature after deployment; add application-deadline persistence and expiry handling in a separate PR |
| Browsing and filtering | Reusable list, search, pagination, and job detail | [Jobs view](../web/src/lib/components/JobsView.svelte), [jobs route](../web/src/routes/jobs/+page.server.ts) | Verify the Bangladeshi source and curriculum workflow against actual stored jobs |
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
