# Student follow-up on jobs

This is a lightweight staff workflow, with no student directory or bulk import.
A reference points to a student in the school's existing system. It is attached
to one job and does not create a student login, verify skills, or calculate fit.
The same student reference can be used on several jobs without sharing profile
edits between them. References differing only in case are treated as the same
student on a given job; use a stable school ID when possible.

## Daily workflow

1. Open **Team review** on an active public job, then **Student follow-ups**.
2. Choose **Link a student**. Enter a stable student ID or profile reference,
   an optional name and HTTP/HTTPS school profile URL, and a reason the job may fit.
   One job may have multiple student references. Duplicate references return 409.
3. Open a follow-up to read/edit its reason, outcome, notes and next follow-up
   date. References stay fixed so audit events remain associated with one identity.
   If linked in error, close the record with a note and add the correct reference.
4. Read the original posting before contacting a student. Review/edit the message
   draft and **Copy message**. Send it through your usual channel, respecting the
   student's contact preferences in your school system. Copying never records
   contact or changes an outcome. No email, WhatsApp or other message is sent by
   this application.
5. After making contact, choose email, phone, WhatsApp, Messenger or other; enter
   the contact time in Bangladesh time and an optional note. **Record manual
   contact** records who did it, when, and how. The first contact moves `suggested`
   to `informed`; later contact preserves an advanced application outcome.
6. Update the outcome as the student responds: `suggested`, `informed`,
   `interested`, `applied`, `interview`, `offer`, or `closed`. `informed` requires a
   recorded contact. Other outcomes may be entered directly, for example when a
   student already applied. Corrections and reopening are allowed and audited.
7. Use **Team workspace → Student follow-ups** to search references/names across
   your authorized jobs, filter outcomes, and find overdue follow-ups. Lists use
   50-row pages. Dates are overdue before today in Bangladesh, except for offer
   and closed outcomes. Set a new date or clear it when the next action changes.

Save outcome edits before recording contact; the UI keeps those two actions
separate so a contact cannot silently discard an outcome draft. Neither copying
nor recording contact saves unsaved message text as an audit message. Add needed
context to the contact note. Contact history describes a staff assertion of
manual contact, not confirmation of delivery or reading by the student.

## Access and history

Only managers and a public job's current assignee can read/write its student
references, notes, contact records and history. Managers may start the workflow
on an unassigned public job. Reviewers use their assigned jobs. Reassignment
transfers the whole job workflow to the new assignee and revokes the old assignee;
unassignment leaves managers with access. Deactivated staff and ordinary users
cannot access it. Private pasted jobs are excluded even for managers.

Every endpoint uses the existing session-cookie gate and private/no-store headers;
API keys cannot access student follow-ups. Student details never enter public job
responses, public search, curriculum badges, the candidate profile or a bot.
Profile URLs are optional HTTP/HTTPS links without credentials; the server does
not fetch them. No student email or phone directory is collected.

The UI displays the latest 100 audit snapshots, newest first, including who
changed the outcome/reason/notes/profile link and each manual-contact channel,
actual contact time and note. The server also records when the assertion was
saved. Older events remain stored. Closed job postings retain their follow-ups
and allow outcome updates/history; new student links and contacts are refused.
There is no hard-delete action; close incorrect references with an explanation.

Each save checks the loaded version. A conflict returns 409 and retains the draft
until explicit reload. Unchanged saves add no audit event. Writes lock the current
staff account and public job before the child record, serializing against staff
deactivation, job reassignment and privacy changes. A contact request has a UUID;
a retry with the same actor/payload returns the current record without adding
another contact. Reusing that UUID with different content returns 409. A new UUID
means a distinct contact assertion; this does not deduplicate messages outside
this application. Invalid/future contact times are refused (five minutes of clock
skew allowed). Historical contact times may be recorded deliberately.

## API and deployment

See [the internal API reference](API.internal.md#student-job-follow-ups). Eight
routes provide the global queue, per-job list/create, individual read/update,
manual contact recording and history. All writes and their audit snapshots commit
atomically. A scoped record/history lookup returns 404 if its ID belongs to
another job or is not accessible.

Apply additive migration `0217_student_job_followups.sql` before the app upgrade,
using the [standard deployment](../ops/hero-jobs/README.md). No worker, source
collection, model service or separate credentials are required. Older app images
may retain the tables during rollback; stop using the new UI when rolling back.

Full student profiles/import, automatic job–student suggestions, automated
notifications, a cross-school student identity directory and the discussion bot
remain separate future features. This workflow implements lightweight job links,
manual-contact records and outcomes (T04–T06), without requiring T03 profiles.
