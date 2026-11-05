# LinkedIn job collection

LinkedIn collection searches worldwide by default. Bangladesh is an optional
location filter, not an eligibility requirement. Public job descriptions flow
through existing normalization, search, source health and team assignment tools.
No LinkedIn account, cookies, paid service or Python worker is required.

## Search scope and configuration

The `linkedin` boardless source is registered by `hero-bootstrap` and included in
the enabled-provider rotation. Each run searches software engineer, frontend
developer, backend developer, Python, data analyst and machine learning. It reads
two pages per term, deduplicates by native posting ID and fetches current details.
Requests are paced at one every two seconds. This is bounded discovery, not the
complete LinkedIn catalog.

Optional runtime variables pass through the tools service's existing `.env` file:

| Variable | Default | Accepted values |
| --- | --- | --- |
| `LINKEDIN_LOCATION` | `Worldwide` | Search hint up to 200 bytes; e.g. `Bangladesh` or `London` |
| `LINKEDIN_KEYWORDS` | Six terms above | JSON array of 1–12 nonempty terms, each up to 200 bytes |
| `LINKEDIN_MAX_PAGES` | `2` | Integer 1–5 per term |

The combined budget is at most 30 listing pages and 300 distinct jobs per run.
Invalid configuration fails before requests. A posting's actual location is
preserved; a search hint does not prove eligibility or remote availability.
Changes to runtime configuration apply to the next tools container. For example,
set `LINKEDIN_LOCATION=Worldwide`, `LINKEDIN_KEYWORDS=["python developer"]` and
`LINKEDIN_MAX_PAGES=1` for a narrower search.

Use `refresh-jobs.sh` for scheduled collection: it holds the shared collection
lock. The provider joins the existing five-minute rotation, not a separate
five-minute full-catalog crawl.

## Identity, edits and closure

Postings retain `source=linkedin`, their native ID and canonical
`https://www.linkedin.com/jobs/view/ID/` URL. Titles, companies, descriptions,
locations and employment types come from current details. Listing publication
dates are retained when available; relative prose is not turned into invented
exact timestamps. Descriptions use the shared sanitizer.

Single-link intake recognizes numeric and title-bearing URLs, including country
subdomains and tracking parameters. It shares the collector's identity: importing
a link and later discovering that posting updates one row. Existing assignment
and follow-up tools work on those stored jobs.

Discovered details are re-read on subsequent runs. Edits use the existing refresh
and search-outbox pipeline. Disappearing from search never closes a posting:
LinkedIn opts out of absence-based sweeps. Only a direct detail 404/410 or the posting's
own “No longer accepting applications” marker closes it. A later readable active
detail can reopen it.

`check-linkedin.sh` runs every six hours, sharing the collection lock. It checks a
shuffled sample of up to 100 open postings unseen for 48 hours, at two-second
intervals. The existing liveness mechanism requires two explicit expired verdicts
before closure. It stops on the first uncertain response. This is bounded stale-job
verification, not guaranteed full coverage per run; it does not refresh old
content outside discovery.

## Failures and operations

Guest endpoints can change or become unavailable. Search failures, malformed cards
and entirely empty discovery fail visibly. Detail refusals (403/429/999), or three
consecutive unreadable details, stop the run. Other unreadable details never
replace stored content or become closure evidence. Login walls, redirects and
footer text are not expiration signals.

Inspect collection logs and existing public `/status` source health for attempts,
errors and last success. Probe outcomes appear in `logs/linkedin-liveness.log`.
Failed runs preserve existing jobs. If public access stops working, pause the
source in the enabled-provider policy and use the moderator manual form with the
original posting URL. Link intake shares the same guest access and can also fail.

## Access alternatives checked on 2026-11-06

- The native Go adapter uses anonymous LinkedIn guest search and detail endpoints.
  Both answered from development and the production host. A complete six-term,
  two-page Bangladesh probe returned 87 readable active postings and three
  unreadable details. This was a probe, not production ingestion. Worldwide
  defaults and non-Bangladesh locations are covered in adapter tests. A subsequent
  live worldwide probe through the Go adapter returned 10 readable postings from
  Singapore, Finland, South Korea and the US, including Apple, AWS, Wolt, Grab and
  JPMorganChase. No production job records were written by these probes.
- [JobSpy](https://github.com/speedyapply/JobSpy) is an MIT-licensed Python
  alternative. An isolated `python-jobspy==1.3.0` test with worldwide search and
  `fetch_description=True` returned five postings with descriptions across Finland,
  Singapore, South Korea and the US. It would add a Python runtime and dependency
  lifecycle; it is not part of this project's production image.
- [Apify's Bebity scraper](https://apify.com/bebity/linkedin-jobs-scraper) offers
  managed public-job collection and structured output. Its advertised starting
  price at review was $1 per 1,000 results; selected options may cost more. It
  needs an account/token and usage budget. It was researched, not run or integrated.
  The [dataset API](https://docs.apify.com/api/v2/actor-run-sync-get-dataset-items-post)
  could support a future adapter if managed collection is chosen.
- LinkedIn's [Job Posting API](https://learn.microsoft.com/en-us/linkedin/talent/job-postings/api/overview)
  is a partner publishing integration, not a general public job-search API.

The native adapter reuses existing Go HTTP clients and ingestion lifecycle. None
of these approaches promises complete worldwide coverage or permanent guest
access. No LinkedIn credentials or paid scraping runs were used.

## Validation

Synthetic adapter tests cover worldwide defaults, optional configuration, both
observed card layouts, pagination/deduplication, canonical links, sanitation and
failure/closure distinctions. Database tests cover link-first then crawl identity,
edits, failure preservation, explicit closure/reopening and search-outbox updates.
Liveness tests distinguish active details, explicit removal, redirects and access
walls. Deployment and production collection remain separate verification steps.
