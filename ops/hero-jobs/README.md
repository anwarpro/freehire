# Internal jobs deployment

This directory owns the deployment for `jobs.programming-hero.org` on DigitalOcean.
The source checkout is `/opt/freehire`; runtime configuration, `.env`, backups,
logs, and collection cursors are kept separately in `/opt/hero-jobs`.

## Deploy a fork revision

```sh
cd /opt/freehire
git fetch origin
git switch codex/internal-deployment
git pull --ff-only origin codex/internal-deployment
sudo sh ops/hero-jobs/deploy.sh
```

The deploy command builds the application and worker tools from this checkout,
backs up PostgreSQL, applies pending migrations, refreshes the running services,
and checks HTTPS health. Database and search data live in named Docker volumes.
It preserves runtime credentials and source registrations on redeployment.
Never use `docker compose down -v`: it removes persistent data volumes.

The initial schema dump clears its PostgreSQL session search path. If deploying
against a completely empty database, the first migration run can stop after
migration 0005. Re-running the migration command starts a fresh session and
resumes remaining migrations; do not baseline or skip unapplied files.

## Add a company using an existing adapter

Register a verified board with its provider, board identifier, and employer name:

```sh
sudo /opt/hero-jobs/add-source.sh greenhouse YOUR_BOARD_ID 'Your company'
```

Board identifiers come from the company's career URL or the provider's public
API. They are not arbitrary search URLs. The upstream `add-board` command checks
configuration but does not prove a board returns jobs; use `harvest-boards` to
probe a candidate list against its source before adding it.

`greenhouse-seeds.json` is a small initial candidate list, not an assertion that
all entries are live. Probe it and persist only boards that validate:

```sh
cd /opt/hero-jobs
docker compose run --rm -T --no-deps \
  -v /opt/hero-jobs/greenhouse-seeds.json:/seeds.json:ro \
  --entrypoint /app/harvest-boards tools \
  --apply -workers 2 -pace 1 greenhouse /seeds.json
```

Boards are stored in PostgreSQL. The source catalog is independent of the Git
repository, and remains in place when application code is updated.

## Bulk import for a larger catalog

Prepare a JSON array of board IDs or objects such as
`[{"board":"company-slug","company":"Company name"}]`, then run:

```sh
sudo /opt/hero-jobs/import-sources.sh greenhouse /path/to/your-seeds.json
```

The importer validates up to 50 candidates per batch and saves each completed
batch. This avoids losing an entire large discovery run when it hits a time
limit. Existing live catalog entries are skipped on repeat imports. Keep
large inventories and harvested data outside Git; commit integration code
and small curated source configurations.

For an ATS company inventory CSV with `name,slug,url` columns, the upstream
`seed-from-inventory` worker converts its career URLs into seed JSON without
writing to the database. Validate those generated seeds with the importer
before collection. No complete hosted Freehire catalog is bundled with the code.

The scheduled `expand-tech-catalog.sh` reuses Freehire's existing public
tech-inventory discovery, treating those URLs only as candidates. It validates
each board against its own ATS before registration. Logs distinguish candidate
counts from verified boards and collected postings. The generated inventory
stays on the server under `state/tech-seeds`, outside Git.

## Add an entirely new platform

Implement a source adapter under `internal/ingest/sources`, register it in
`sources.All`, and follow that package's instructions and tests. Commit the
integration to this fork and redeploy. A new adapter still needs its catalog
entries and any required API credentials. LinkedIn, Indeed, and Bdjobs are not
made available merely by registering their names.

## Collector coverage and schedules

`hero-bootstrap` produces `source-inventory.json`, recording every compiled
adapter, whether it requires board IDs, and whether credentials are configured.
It also registers the available boardless sources through the upstream catalog
writer. Repeated initialization does not duplicate live entries.

- Collection: every five minutes, one provider per run, oldest attempt first.
- Application-deadline closure and incremental search indexing: every two minutes.
- Supported board discovery: every two hours, one provider per run.
- Statistics/company index: every six hours.
- Tech-company catalog expansion: daily at 22:45 UTC (04:45 Bangladesh).
- Database backup: daily at 20:15 UTC (02:15 the following day in Bangladesh).

Collection is sequential and memory-limited for the current 4 GB server.
`enabled-providers.txt` selects the tech-focused collection rotation. It keeps all
compiled adapters available while prioritizing worldwide tech listings. Add a
provider name there and redeploy to include its registered boards. The initial
target is 50,000–100,000 searchable listings; that target is not a claim that the
catalog has already reached it.

Being compiled, registered, or scheduled does not mean an upstream site is
reachable or that collection has succeeded. Check `logs/collector-results.log`,
`logs/collect-<provider>.log`, and `logs/discovery-<provider>.log`.

Some automatic discovery providers enumerate thousands of candidates and may
exceed the 30-minute budget before their validated boards are persisted. For
large catalogs, use bounded, verified seed batches rather than repeating the
same oversized discovery run. Start by adding the employers relevant to your
team; scale memory and collection capacity based on measured load.

## Measure the tech-job target

The database can include non-technical roles advertised by tech companies.
Measure the 50,000–100,000 target with the technical facet, rather than the
unfiltered catalog count:

```sh
curl 'https://jobs.programming-hero.org/api/v1/jobs/search?is_tech=tech&limit=1'
```

The response's `meta.total` is the matching searchable count. The facet value
is `tech`, not the boolean `true`; unknown classifications are excluded.

## Secrets, backups, and optional features

Real credentials belong only in `/opt/hero-jobs/.env`, mode 600. Never commit
that file, database backups, scraped data, logs, or SSH keys. Only the HTTPS
proxy publishes container ports; the database, Redis, backend, and Meilisearch
remain private to the Docker network.

Backups are local PostgreSQL archives retained for seven days. Configure
an off-server backup destination for recovery from loss of the Droplet.
CV uploads require persistent S3-compatible storage. LLM features, OAuth,
mail, and credentialed collectors require their respective credentials.
Website access protection remains deferred at the owner's request.

## Preserve integrations when updating upstream

Keep `origin` pointing to your fork and `upstream` pointing to `strelov1/freehire`.
Fetch upstream, merge its changes into your deployment branch, resolve conflicts,
run the required checks, push to your fork, and redeploy. Do not reset the fork's
branch to upstream: that would discard your integration commits.

## Bangladesh company career pages

See the [collection workflow and verified shortlist](../../docs/bangladesh-company-careers.md).
Moderators can preview, enable and pause company career URLs from
`/moderation?tab=careers`, including custom HTML configuration. Six supported
methods and four explicit fallbacks are tracked separately. Managed company
boards receive bounded priority in the existing five-minute collection rotation;
raw failures retain per-board logs. The full deployment verifies/enables the
supported shortlist and records initialization separately from service health.
