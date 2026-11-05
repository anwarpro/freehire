# PH Jobs: implementation baseline

Reviewed locally on 2026-11-05. This is a code audit, not a live deployment verification.
Each independently usable feature should have its own PR, based on the internal
deployment branch until the fork establishes another integration branch.

## Existing capabilities and next steps

| Capability | Finding | Evidence | Next step |
| --- | --- | --- | --- |
| Manual job intake | Reusable backend and public submission form; direct moderator publishing now uses the same form | [Moderation API](../internal/api/handler/jobs_moderation.go), [submission form](../web/src/lib/components/SubmitView.svelte), [moderation page](../web/src/lib/components/ModerationView.svelte) | Verify the publishing flow on staging; implement manual editing and date/deadline support as separate features |
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
