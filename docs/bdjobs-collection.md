# BDJobs collection

The `bdjobs` adapter reads the public API used by the official
[BDJobs IT/Telecommunication search](https://bdjobs.com/h/jobs/?fcatId=8).
The initial scope is that complete category, including its English and Bengali
postings, rather than every occupation on BDJobs. No account, partner credential,
browser automation or third-party scraping service is required by this method.
The source is a multi-employer aggregator with one boardless catalogue identity.

## Verified access

The public web client uses these endpoints, verified on 5 November 2026:

- `GET https://api.bdjobs.com/jobs/api/v1/JobSearch/GetJobSearch?category=8&pg=1&rpp=50&ToggleJobs=true`
- `GET https://api.bdjobs.com/jobs/api/v1/JobSubsystem/jobDetails?jobId=JOB_ID&ln=1&IsCorporate=false`

`category=8`, `pg` and `rpp` are the current API parameters. The legacy page
parameter `fcatId=8` does **not** filter the API. `ToggleJobs=true` is the public
English-interface request; Bengali-titled jobs also occur in its results.
The initial listing reported 311 rows across seven pages. The final live adapter
pass read 311 distinct postings: 310 readable active jobs and one image-only
advert (`282859`) explicitly unreadable. Its original scanned advert remains on
BDJobs; no OCR or invented body is substituted. Anonymous search and full-detail
access were also verified from the production host without changing stored jobs. BDJobs can repeat a
native job ID in regular results and in rotating promoted rows, so this count
is not a promise of 311 distinct readable jobs. Inventory changes over time.
These are the site's own public web endpoints, not a versioned partner-feed
agreement; access or schema changes are collection failures that require review.

## Import and lifecycle

The collector first checks every listing page against its reported total and page
count. Missing metadata, missing/null rows, short pages, repeated whole pages,
changing totals, HTTP failures and the 100-page ceiling fail the entire pass.
A positively verified zero count is a valid empty feed. Repeated individual
postings and promoted copies are deduplicated by the numeric BDJobs ID.

Every listed detail is re-read on each refresh, at a shared maximum of one request
per second across listing and detail calls. This observes body and deadline edits,
not just new jobs. The normalized posting preserves:

- Native ID and original `https://bdjobs.com/h/details/ID` link.
- Employer and title from the detail response, including Bengali text.
- Responsibility, context, requirement, education and benefit HTML, sanitized
  through the shared description sanitizer.
- Stated location, employment type and work arrangement. An unknown location is
  not replaced with a guessed country or a claim that the job is remote.
- The listing's `publishDate`, when parseable, as the posted time. The detail's
  formatted `PostedOn` label is not substituted for that timestamp.
- The `DeadlineDB` calendar date. Applications remain open throughout that date
  in Bangladesh: a 5 November deadline expires at 6 November 00:00 Asia/Dhaka
  (5 November 18:00 UTC). This is separate from posted-date parsing.

An elapsed source deadline, explicit `Closed` flag, or the detail API's exact
`statuscode=1`, `message=data not found`, empty-data answer emits a removal.
Timeouts, refusals, locked early-access adverts, mismatched identities, malformed
successes and image-only bodies are unreadable details, never inferred removals.
An entirely unreadable detail pass fails; partial unreadable results are counted
and withhold unseen closure permission. A complete readable catalogue can use the
existing 48-hour unseen grace; failed or incomplete passes cannot close unseen jobs.

The normal ingest pipeline supplies boardless identities (`:ID`), stable public
slugs, repeat-import updates, dictionary skill tagging, aggregator suppression,
search-outbox writes and deadline expiry. No parallel job store or search API is
introduced. An IT-category signal accompanies these postings; skill and curriculum
matching still use the existing dictionaries and later curriculum workflow.

## Enable, collect and inspect

A deployment runs `hero-bootstrap --apply`, which registers this boardless source
through the existing catalogue writer. Repeating initialization adds no duplicate.
`bdjobs` is included in `enabled-providers.txt`, so it joins the general provider
rotation. The scheduler wakes every five minutes and runs one collector under the
shared collection lock; this is not a promise to crawl each provider every five
minutes. Managed company pages retain their existing priority.

On the deployment host, from `/opt/hero-jobs`:

```sh
docker compose run --rm -T --entrypoint /app/hero-bootstrap tools --apply
./refresh-jobs.sh
./index-jobs.sh
```

The rotation chooses the oldest eligible provider; `refresh-jobs.sh` may select
another provider first. To diagnose BDJobs specifically while no other collector
is running, use the existing lock and bounded worker configuration:

```sh
flock -n state/collect.lock docker compose run --rm -T \
  -e GOMEMLIMIT=900MiB -e INGEST_RUN_TIMEOUT_SECONDS=1200 \
  -e INGEST_SWEEP_RESERVE_SECONDS=120 \
  --entrypoint /app/ingest tools bdjobs
./index-jobs.sh
```

Inspect `logs/collect-bdjobs.log`, `logs/collector-results.log`, and the existing
[source health page](https://jobs.programming-hero.org/status). Source discovery,
counts and links use the existing [source catalogue](https://jobs.programming-hero.org/sources).
Successful, indexed jobs are available through the BDJobs source filter and
`GET /api/v1/jobs/search?source=bdjobs`. Public counts can be lower than raw imports
because the shared publication and duplicate rules still apply. Source failures
retain existing jobs and follow the shared cooldown/retry policy.

## Verification and release boundary

Adapter tests cover advancing pagination, individual page overlaps, promoted copies,
requirements-only bodies, stale listing deadline extensions, safe emptiness, malformed
or incomplete lists, page limits, source fields, HTML sanitization, Bengali text,
Bangladesh deadlines, explicit removal and unreadable details. A database integration
test runs this adapter through the real ingest store to verify source identity,
stable slugs, search queueing, repeat imports, deadline-only changes, failed-crawl
preservation and closure.

Local verification and a PR do not count as production deployment. Production
acceptance must additionally verify catalogue registration, a successful collection,
stored jobs, search indexing and source health after merging and deploying.
