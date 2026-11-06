# Team job discussion

Open **Team workspace → Discuss jobs**. This English-only mode reuses the existing
assistant's session store, bounded tool loop, streamed replies, retry and stop
controls. It is a read-only discussion of collected public jobs and the current
curriculum rules. It creates no student directory and sends no messages.

## Using the bot

Choose an existing conversation or explicitly start a new one. Examples:

- Show recent React jobs from LinkedIn.
- Which Python jobs overlap with our courses?
- Compare the requirements of these two public job slugs.
- Is this posting closed, and when was it last seen?

The model receives source URLs, this installation's job links, posting and
last-seen dates, closure/deadline fields and literal requirement evidence.
Successful tool results also supply a separate **Verified job sources** list in
the UI; those links do not depend on what the model writes. A closed posting is
labeled in that list. Job descriptions are treated as untrusted source text.

The five tools are `facets`, `search_jobs`, `get_job`, `curriculum` and
`course_jobs`. Search supports existing source/skill/location facets and optional
`posted_within_days` (1–365). It returns at most 20 jobs and rechecks authoritative
privacy/closure before returning search-index hits. `get_job` reads a public job
and computes current automatic curriculum evidence using the shared matcher on
the original description (first 50,000 characters, with truncation reported).
`course_jobs` pages persisted automatic suggestions (10 per page), with mapping
version, pending-update markers and scan progress. Read individual jobs for fresh
evidence; a partial scan or queued edit means a missing course result is not proof
that no matching job exists. Closed/all course results must be requested explicitly.

Automatic curriculum suggestions are unreviewed skill overlap, not student
eligibility or a human reviewer decision. Private reviewer labels/notes, student
references/contact history, candidate profiles, inbox, CVs and account editing
are outside this tool registry. Human decisions remain in Team review. A model
cannot call the candidate assistant's writing tools from this mode.

Each user owns their conversations. Session lists use 50-row pages, and an
explicit confirmation deletes a conversation. Refresh/reload replays the saved
transcript. After an interrupted stream, reload before sending again: the server
may still be finishing the earlier turn. Stop explicitly cancels it. Changing
team tabs stops the component's active turn.

## Access and usage

Every `/team/bot` endpoint requires an active team member's session cookie and
returns private/no-store responses, including streamed replies. Ordinary users,
deactivated staff and API keys cannot use it. Session IDs are scoped to the owner
and the `team_jobs` preset. Candidate assistant endpoints cannot read, delete,
run, cancel or extend these sessions, and team endpoints cannot open candidate
sessions. Each tool checks current team access again.

The existing paid-AI rule remains: model turns require Pro or Ultra entitlement
and use the caller's existing daily assistant allowance. Team membership alone
does not grant model access. Usage is attributed under the `team_jobs` preset.
No entitlement or production model settings are changed by this feature.

The UI reports configuration and entitlement separately. Existing conversations
remain readable while a model is unconfigured or paid access has expired. A model
client being configured does not prove the gateway accepts its key/model; live
provider errors are still possible and are reported as failed turns.

## Gateway configuration

The user selected this OpenAI-compatible gateway base URL:
`https://neptune.open.arc9.pro/v1`. The intended model was described as DeepSeek
v4 Flash, but its exact gateway model ID has not been supplied or verified.

Configure credentials privately in `/opt/hero-jobs/.env` (mode 600), never in Git
or chat. The existing configuration is reused:

- `LLM_BASE_URL`: the selected gateway URL.
- `LLM_API_KEY`: the gateway credential.
- `LLM_MODEL`: the default model ID required by the existing LLM configuration.
- `ASSISTANT_MODEL`: the exact tool-capable model ID for assistant turns; defaults
  to `LLM_MODEL` if omitted. Different extraction chains can keep their own models.
- `ASSISTANT_MAX_STEPS` and `ASSISTANT_MAX_PROMPT`: existing bounded-loop settings.

The gateway must support OpenAI-compatible chat completions with function/tool
calling and streaming. The URL alone does not establish that the described model
is available. An unauthenticated model-list request returned HTTP 401; no
inference was performed or billed during implementation. A
private key and exact model ID are still required for live answer verification.
If internal-team allowances should replace Pro/Ultra access, that is an explicit
follow-up policy change rather than an implicit bypass of the paid gate.

## API and deployment

See [the internal API reference](API.internal.md#team-job-discussion). Status,
conversation list/create/read/delete, send/retry and cancel are under `/team/bot`.
Apply migrations 0218–0219 before deploying app/web. The first expands the session
preset CHECK; the second validates it and builds indexes concurrently outside a
transaction. Use the [standard deployment](../ops/hero-jobs/README.md). Retain the
additive preset/schema on binary rollback; old UIs do not offer team sessions.

Regression tests cover private slug reads (including the general assistant's
`get_job`), stale/private/closed search hits, ownership and cross-surface isolation,
team revocation, real entitlement rejection, streamed and replayed evidence,
partial/empty course searches, current rules and English-only instructions. UI
checks cover configuration state, explicit creation, source links, interrupted
streams, access revocation and deletion confirmation. Scripted model tests verify
transport and tool boundaries; they do not establish live DeepSeek answer quality.

Before enabling real use, verify a small real question set against the configured
gateway: source citations, no-result honesty, stale/closed state, course evidence,
unknown salary/requirements, and job text containing malicious instructions.
Student suitability discussion (B04), automated notifications and private team
workflow tools remain later features.
