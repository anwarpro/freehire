# Bangladesh company career collection

This feature collects company career pages through the existing ingestion,
identity/deduplication, closure and incremental search-indexing pipeline.
It includes a moderator source manager at `/moderation?tab=careers` and a curated
ten-company onboarding shortlist. **Six collection methods were verified; four
entries have explicit manual fallbacks.** This is not a claim of ten automated
collectors, or of BDJobs/LinkedIn collection.

## Verified source shortlist

Checked against the companies' public career pages and their own application
feeds on 2026-11-05. Counts below describe the upstream verification snapshot,
not persisted/searchable job counts: catalogue policy and deduplication still apply.

| Company / official career page | Collection method | Verification snapshot / limitation |
| --- | --- | --- |
| [Vivasoft](https://vivasoftltd.com/career/) | Open Apply Now links → Novara public job API | 5 active postings; exact publication and application-deadline timestamps. Closed tiles are excluded |
| [Cefalo Bangladesh](https://career.cefalo.com/) | Cefalo public jobs API; HTML detail for active roles | 0 active, 3 explicitly expired postings. The landing page still advertises these under Current Openings; the API's expiry flag is authoritative |
| [Enosis Solutions](https://careers.enosisbd.com/) | Existing Pinpoint adapter, custom career hostname | Valid empty `data` array, 0 active postings; an empty structured feed is distinct from a failed request |
| [Dynamic Solution Innovators (DSi)](https://app.hrythmic.com/recruit/openings/company/dsinnovators/) | Configured server-rendered Hrythmic listing/detail | 1 active posting; original per-job link and full description verified |
| [Ollyo](https://ollyo.com/careers/roles/) | Configured server-rendered listing/detail | 3 active postings; original per-job links and full descriptions verified |
| [weDevs](https://wedevs.com/career/) | Career-page links → lounge.wedevs.com job descriptions | 3 active postings; additional detail host explicitly configured. Location stays unknown unless the source states it |
| [Brain Station 23](https://erp.brainstation-23.com/jobs) | Manual fallback | Public career access could not be verified (ERP jobs endpoint refused the probe). No collector is enabled on that evidence |
| [SELISE](https://selisegroup.com/about-us/#selise-career) | Manual fallback | Probe returned a challenge page rather than a vacancy list |
| [BJIT](https://bjitgroup.com/career) | Manual fallback | Initial HTML says no vacancies before JavaScript loads its API. A complete public feed has not been verified; that HTML is not accepted as an empty feed |
| [Mediusware](https://mediusware.com/career) | Manual fallback | Job data loads using JavaScript; a complete public feed has not been verified |

The next source areas remain [BDJobs](https://jobs.bdjobs.com/) and
[LinkedIn Jobs](https://www.linkedin.com/jobs/), under their own major deliveries.
These URLs are research targets, not connected collectors.

## Add another company without a code change

1. Sign in as a moderator and open **Moderation → Company career pages**.
2. Enter the company name and public HTTPS career URL, or choose **Configure**
   on a supported shortlist entry.
3. Choose **Preview collection**. Recognized ATS pages reuse the existing adapter;
   custom pages may expose schema.org JobPosting data directly.
4. For custom HTML, expand **Configure custom HTML selectors**. Supply the job-link
   selector and the full-description selector; configure title, location, posted
   date and next-page selectors when needed. Cross-host detail links need an
   explicit hostname allowlist. Only use a fallback location the source confirms.
5. Inspect the active/expired counts and sample job titles. **Enable collection**
   verifies again and writes a pending board through `boardcatalog.Inserter`.
   A stale preview cannot enable an edited draft in the UI.
6. Check the collection status after the scheduled worker runs. **Pause collection**
   retires the board without deleting job records. To replace settings, pause first
   and configure the source again; stable job URL identities survive re-enabling.

Unsupported JavaScript-only sites need a verified adapter/feed. Supplying their
URL alone does not turn them into working collectors. Manual publishing remains
available while access is investigated. A formerly blocked shortlist URL can be
verified with a custom configuration once its public HTML becomes readable.

## Crawl safety and lifecycle

Custom HTML reads at most 20 listing pages and 200 jobs. Pagination loops, caps,
failed listing pages and unknown empty pages fail verification. A zero-job HTML
page succeeds only with its configured, source-verified empty-list notice; text
inside scripts/head/style is not an empty-list notice. Some sites do not publish
an observable empty notice yet: after their last vacancy disappears, collection
will require review rather than claiming a successful empty feed.

Per-posting unreadable markers use the normal pipeline's protection against
unproven closure; a crawl that reads none of its listed jobs fails. Complete
listing walks participate in the existing board-scoped 48-hour unseen close.
Explicit removals/expiry flags close retained postings through the same pipeline.
Vivasoft's exact deadlines are persisted and use the existing two-minute expiry
worker and application guards. Unknown source deadlines preserve a previously
known deadline; a moderator can clear/correct one through the job editor.

Custom job identity hashes the canonical original detail URL and is namespaced by
its career board. Title/description changes update the existing job. URL changes
are new source identities and the old posting ages out normally. Original links
remain on the shared job/detail views. No static job fixtures are imported into
production.

Outbound requests use the existing DNS/redirect-aware SSRF guard and body caps.
Only public HTTPS URLs without credentials or nonstandard ports are accepted by
the setup flow. Selectors are parsed CSS; no JavaScript executes. Preview/enable
are moderator-gated and use the authenticated outbound-fetch budget with a
30-second deadline. Raw upstream errors are not returned by the moderator API.

## Scheduling and failure visibility

The existing five-minute `refresh-jobs.sh` rotation first selects a due managed
company board, oldest attempt first, then falls back to the existing provider
rotation. A managed source is attempted at most once per hour. The existing
collection lock prevents parallel collectors. Attempts are stamped even if a
worker fails early, so a broken source cannot monopolize every run. Worker
cooldown/quiet backoff may delay an actual HTTP fetch.

`next-collection.py` returns provider/board/region as data, never shell code.
Scoped runs pass the existing `--board`/`--region` arguments to ingest. Company
sources do not require edits to `enabled-providers.txt`. The general source
rotation remains governed by that policy file.

The moderator view shows pending/healthy/failed/paused, last attempt, last success,
consecutive failures, saved count and cooldown. Raw errors stay in operator logs.
`logs/collector-results.log` identifies each run's exit code and log path. Scoped
runs retain separate `logs/collect-<provider>-<board-hash>.log` files, so one
company does not overwrite another's failure details. Retired board rows stay in
the manager; retirement uses the existing catalogue behavior for health cleanup.

## Deploy and initialize

Use the [full deployment procedure](../ops/hero-jobs/README.md). Apply migrations
0212–0214 **before** the new app/tools: nullable board configuration and the
expanded, separately validated source-management surface constraint. Rebuild
app, web and tools. No search reindex or existing-board backfill is required.

The deployment installs the selector script alongside `refresh-jobs.sh` and
attempts initial shortlist verification/enabling once. Successful completion sets
`state/career-shortlist-initialized`. Remote failures leave that flag unset and
are logged in `logs/career-shortlist-bootstrap.log`; they do not fail healthy
application deployment. Enabled sources still appear in the source manager.

To inspect/retry explicitly from `/opt/hero-jobs`:

```sh
# Public-source verification, no catalogue writes.
docker compose run --rm -T --no-deps --entrypoint /app/company-careers tools
# Reverify and enable supported entries; existing live boards are not duplicated.
docker compose run --rm -T --no-deps --entrypoint /app/company-careers tools --apply
```

The four fallbacks are recorded as fallbacks and are never enabled by this command.
Live deployment and a signed-in moderator check remain separate from local
implementation/PR completion. Keep the expansive schema on application rollback.
