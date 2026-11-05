# Internal team accounts and job assignment

This delivery implements T01/T02 as one independent feature. Existing global
moderators and administrators manage reviewers at `/team`. Active reviewers see
**My assignments**. Public visitors and ordinary accounts have no team access.

## Manager workflow

1. Sign in with an existing moderator/admin account. Open **Team workspace** in
   the account menu, then **Team members**.
2. Add a reviewer with a name, email and initial password (8–72 bytes), or choose
   **Use an existing verified account**. New accounts use the normal password
   registration policy, retain the global `user` role and start with an unverified
   email. Existing accounts keep their password. Duplicate accounts/members are
   rejected. No invitation email is sent; share initial credentials privately.
3. Under **Assignments**, search for a posting and open **Team review**. The same
   control appears on public job detail pages, including closed postings. Select
   an active member, review status, private notes and an optional follow-up date.
4. Reassign by choosing another active member, or choose **Unassigned**. Managers
   can filter all/mine/unassigned review queues, status and overdue follow-ups.
5. Use **Manage reviewer** to rename, deactivate or reactivate team access.
   Deactivation takes effect for subsequent team requests even with an existing
   session. Assignments and history remain visible to managers for reassignment.
   The user's public account remains usable. Global manager privileges are
   managed through the existing account-role process, outside this screen.

Reviewers sign in normally, open **My assignments**, and use **Team review** to
record progress. They can update status, notes and follow-up on their current
assignments. They cannot create team members, read the roster, change assignees,
or inspect other assignments. After reassignment, the former reviewer loses
access to that review and its history. Staff can enter their workspace without
completing candidate CV onboarding. Password changes use the existing Security
screen.

## Review behavior and privacy

- One review per public job, independent of the public posting. Private pasted
  jobs are excluded. Closing a posting preserves its review and queue entry.
- Statuses: `pending`, `in_review`, `reviewed`, `not_relevant`. Notes are plain
  text, up to 5,000 characters. They do not enter job descriptions, public APIs,
  search indexing or analytics events.
- Follow-up is an optional calendar date, not a notification schedule. Pending
  and in-review entries become overdue after that date in Bangladesh. Reviewed
  and not-relevant entries are excluded from the overdue filter.
- Saving uses the loaded version. Conflicting saves return 409 and preserve the
  local review draft; **Reload latest (discard draft)** explicitly replaces it.
  An unchanged save does not create an extra history entry.
- Review updates and audit snapshots commit together. Snapshots include actor,
  assignee, status, notes, date, version and time. Membership changes also have
  history. UI/API history reads the latest 100 events; earlier events remain
  stored. Names resolve from current accounts, so name changes also update old
  history labels. Deleted accounts display a fallback label.
- Team endpoints require the session cookie, refuse API keys and return
  `Cache-Control: private, no-store`. The page's server-rendered shell contains
  no private team data. Membership is checked per request; write locks serialize
  deactivation and assignment, and version checks prevent lost updates.
- Queue pages contain 50 reviews with `meta.has_more` and offset pagination.
  The roster supports up to 500 visible staff accounts. Unassigned queue means
  existing unassigned reviews; finding jobs without reviews uses public search.

The [internal API reference](API.internal.md#team-job-reviews) documents all nine
`/api/v1/team/*` endpoints. Login and `/auth/me` expose optional `team_role` for
navigation; it is not an authorization credential.

## Deployment and acceptance

Apply additive migration `0215_team_job_reviews.sql` before the app/web update,
using the [existing deployment procedure](../ops/hero-jobs/README.md). It creates
four new tables and indexes; no public job or account data backfill is needed.
No email, model, search index or scheduler configuration is required. Existing
moderator/admin accounts can immediately manage the workspace. Do not bootstrap
reviewers with sample credentials in production.

After deployment, verify with a real manager and a designated test reviewer:
create/enroll, assign a public job, see it in My assignments, save progress,
inspect history, reassign, then deactivate/reactivate team access. Check that
ordinary users and former assignees cannot read notes. A rollback to the previous
app/web release may leave the additive tables in place to preserve reviews.

Student profiles, job–student matching, notification records and outcomes are
separate T03–T06 work. This delivery sends no messages to students. LinkedIn
collection remains the next major delivery after team assignment is deployed.
