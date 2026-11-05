# Curriculum matching and job badges

This feature covers editable curriculum mapping (Q02), manual badges (Q06) and
explainable automatic suggestions (M01) as one workflow. It is private to the
internal team at **Team workspace → Curriculum jobs**. It uses job requirement
text, not a model, and does not send student messages or claim student eligibility.

## Approved editable defaults

The user approved these track groupings on 2026-11-06. The starter aliases are
editable rules, rather than a claim to reproduce a complete lesson syllabus.
All six tracks initially need one distinct matching skill; repeated aliases of
the same skill do not increase that count.

| Stable ID | Initial label | Starter skill groups |
| --- | --- | --- |
| `l1` | L1 · Frontend fundamentals | HTML, CSS, JavaScript, TypeScript, React, accessibility |
| `l2` | L2 · Backend development | Node.js, Express, databases, APIs, backend engineering |
| `python` | Python · Software engineering | Python, Django, FastAPI, Flask, testing |
| `ai-ml` | AI / ML | Machine learning, deep learning, ML frameworks, language models, computer vision |
| `data-analysis` | Data analysis | Data analysis, pandas/NumPy, BI tools, SQL, spreadsheets |
| `competitive-programming` | Competitive programming | Explicit competitive programming, ICPC/Codeforces, algorithms/data structures |

Tracks may overlap. For example Python and pandas can suggest both Python and
data analysis. Competitive programming is a skill signal and does not independently
prove suitability for a software engineering job.

Managers open **Edit curriculum mapping** to rename badges, change skills/aliases,
adjust the minimum distinct-skill count, disable tracks or add tracks. Rules use
one line per skill: `Skill name | alias, alias`. Existing track IDs cannot be
removed or renamed; disable a track to preserve history. There are at most 12
tracks, 30 skills per track and eight aliases per skill. Labels, skill names and
aliases are bounded. Alias commas/pipes are reserved for the editing format.

A changed mapping creates a new revision and audit snapshot, then starts a
bounded catalogue rebuild. Unchanged saves do not create revisions or rebuild.
Concurrent edits return 409; the UI preserves the draft until **Reload mapping
(discard draft)** explicitly replaces it. Managers can inspect the latest 30
mapping snapshots, including the initial defaults; older snapshots remain stored.

## Evidence and reviewer workflow

1. Filter **Curriculum jobs** by track, source, review state and active/closed
   posting state. Managers browse all public postings. Reviewers see current
   assignments only. The source menu reuses the existing source catalogue.
2. Open **Team review → Curriculum badges** from a result, assignment or public job
   page. Opening this panel refreshes that one job immediately, even while a
   background rebuild is still running.
3. Read the suggested tracks and evidence. Each matched skill includes the
   literal alias, an excerpt and its context: **Required**, **Preferred**, **Job
   title**, or **Mentioned; requirement strength unspecified**.
4. Keep or correct the selected tracks, add private curriculum notes, and save.
   **Select suggestions** copies the current suggestions into the draft; saving
   confirms the selected labels. An empty selection records a reviewed job with
   no relevant track. **Restore automatic suggestions** clears the human override.
5. Use **View curriculum history** to inspect the latest 100 snapshots with actor,
   selected IDs, rationale, review revision, mapping revision and time. Earlier
   events remain stored.

Aliases are literal words/phrases with Unicode boundaries, not regular expressions.
For example SQL does not match NoSQL. Explicit negative phrases such as “Python
is not required” are suppressed. Standard requirement/preference headings carry
their context to following lines; an unqualified mention never becomes a required
skill. The strongest observed context is retained once per skill. HTML is rendered
to plain text and evidence is displayed as escaped text. Matching examines at most
the first 50,000 description characters; longer descriptions carry a visible warning.

This is deterministic overlap matching, not full natural-language interpretation.
Complex alternatives, uncommon wording, seniority, salary, work authorization and
student experience require reviewer judgment. The original posting remains the
source of truth. A suggested badge is visibly unreviewed and is never presented
as approved student fit.

## Review preservation and privacy

Human labels override suggestions, including an explicit empty set. Automatic
refreshes update evidence without overwriting the human decision. A changed job
input or mapping revision marks an earlier review stale. Labels stay visible with
**evidence changed; review again**, and the reviewer can reconfirm/correct them.
Queued source changes also appear in the **Needs another review** filter.

Saving requires the loaded review revision, mapping revision and input hash.
If the job, mapping or review changed, 409 prevents stale evidence from being
approved. Saving a genuinely unchanged review adds no event. An automatic reset
and an explicit reviewed empty set are different states.

All `/team/curriculum*` routes require a session cookie, check current membership
and use `Cache-Control: private, no-store`. API keys, ordinary accounts and
anonymous callers cannot use them. Only managers edit mappings. Reviewers may
read/write evidence, notes and history only for their current assignments.
Reassignment and deactivation revoke that access. Private pasted jobs never
appear in the projection or team results. Closed public postings keep their
curriculum reviews. Curriculum labels and notes do not alter public job data,
public search documents, assignment notes or student records.

## Background processing and deployment

Apply additive migration `0216_curriculum_matching.sql` before updating app/tools.
It creates configuration/history, projection/review history, queue and scan-state
tables, plus a small job-write trigger. No catalogue-wide matching or backfill runs
inside the migration. Existing global managers can use the UI immediately after
migration and deployment.

The trigger queues new jobs, title/description edits, privacy changes and reopened
postings atomically with their writes. It therefore covers company careers,
BDJobs, LinkedIn, manual intake and other existing adapters without maintaining
separate ingestion hooks. A public-to-private change removes its projection.
Existing open public postings are seeded through bounded primary-key pages; older
closed postings can be computed when their team panel is opened or they reopen.

`cmd/curriculum-match` fairly alternates dirty entries and initial/revision seed
pages. It processes up to 2,000 jobs within a 75-second budget per run. A session
advisory lock and the host script's flock prevent duplicate worker runs. Per-job
transactions lock the job before reading configuration and updating the projection,
then remove the queue entry. An edit cannot be consumed without its matching input
being read. Retries are idempotent, cancelled passes resume safely, and human
review fields are never changed by this worker.

The tools image includes the binary. `match-curriculum.sh` runs every two minutes
and pauses during deployment; it uses its own lock so source collection can
continue. A normal initial pass can be started with:

```sh
sudo /opt/hero-jobs/match-curriculum.sh
```

Inspect `logs/curriculum.log` and the UI's mapping revision, initial-scan state,
processed-check count, last run and sanitized error. The count represents checks,
not a count of unique catalogue jobs. Matching is eventually consistent: projected
jobs appear during initial/revision backfill, and queued edits are flagged until
processed. Opening a review refreshes that job synchronously. Track filtering uses
an indexed persisted projection, not a scan or filter of only the current page.
The API performs no full catalogue count.

The [internal API reference](API.internal.md#curriculum-matching) documents the
seven routes. See the [deployment procedure](../ops/hero-jobs/README.md). Rollback
can retain all additive tables and review history; disable the new matching cron
when restoring tools that lack its binary. Pending writes can be drained after a
forward deployment.

## Validation and remaining workflows

Tests exercise literal aliases, negation, qualifiers, distinct-skill thresholds,
long evidence excerpts, disabled tracks, database queue/seed behavior, concurrent
source edits, reopening, privacy removal, reviewer corrections, explicit no-fit,
reset, mapping rebuilds, conflicting saves, audit and membership boundaries.
UI tests cover filtering/pagination, manager editing, draft preservation, worker
progress and revoked access. Desktop/mobile QA uses synthetic local fixtures;
production rollout and initial matching are a separate deployment step.

Student profiles, job–student links, notification tracking, student suitability
suggestions and the job discussion bot remain separate later workflows. This
feature makes no model calls and sends no messages.
