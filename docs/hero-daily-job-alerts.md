# Daily personalized job alerts for PH Jobs

The existing profile alert builds a saved search from the user's role, seniority,
skills, location/remote preferences, and exclusions. Profile edits refresh that
search through the existing profile-alert sync. The existing notification worker
matches distinct searches, deduplicates jobs, honors daily delivery times, skips
closed/stale jobs, retries failures, and records delivered digests in the user's
notification history. No new matching service or student-data copy is introduced.

This delivery adds the missing deployment schedule and runner. `/app/notify`
already ships in the base application image inherited by the tools image; no
duplicate worker build is needed. A link beside the profile-alert toggle takes
users directly to the existing daily/time settings.

## Student setup

1. Save skills, role/experience, and job-location preferences in the existing
   profile. Google sign-in is optional; any existing account can use this flow.
2. In **Notifications → Search alerts**, enable **Jobs matching my profile**.
   This explicitly subscribes the account's email; no one is enrolled by deployment.
3. Follow **Choose daily digest and delivery time**, choose **Daily digest**, and
   choose the time in Notification settings. Daily frequency applies to all of
   this account's search alerts; reminders keep their existing timing.
4. Confirm the profile timezone, for example `Asia/Dhaka`. The existing worker
   uses UTC if none is set; this deployment does not silently rewrite timezones.

Users can disable the profile alert, remove its email subscription, or unsubscribe
through email preferences. Existing alert matches remain deterministic search
matches, not a claim of complete suitability or an AI recommendation score.

## Configure email delivery

The deployment currently has neither email credentials nor a notification-worker
schedule (audit: 2026-11-15). Before activation, configure these in the protected
`/opt/hero-jobs/.env`, preserving the existing file and keeping permissions `600`:

- `AWS_REGION`: the region for the configured SES sender.
- `NOTIFY_EMAIL_FROM`: the verified sender address used for job notifications.
- AWS credentials through the SDK's existing credential chain, such as
  `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`, plus `AWS_SESSION_TOKEN` for
  temporary credentials. A role-based setup is also supported by the existing SDK.

The sending identity/account must permit the intended recipients and SES email
delivery. Confirm sender configuration and account restrictions before activating.
Do not place populated credentials in this repository, PR, or chat. The tools
service already reads `.env` afresh for each run. Existing `MEILI_MASTER_KEY`,
`JWT_SECRET`, and database configuration are also required.

## Deploy and activate

Use the [normal deployment procedure](../ops/hero-jobs/README.md). It installs
`notify-jobs.sh` and the cron entry but leaves delivery disabled unless the
persistent runtime marker already exists. The marker is outside Git and remains
unchanged on later deployments.

After configuring email and reviewing controlled opt-in test recipients, an
operator activates delivery:

```sh
cd /opt/hero-jobs
touch state/job-alerts.enabled
./notify-jobs.sh
```

**The runner sends real messages to due, subscribed recipients once enabled.**
Review existing subscriptions before activation; this is not a dry-run command.
It also processes any existing opted-in non-email subscriptions through the
unchanged worker. This implementation/PR work does not activate production or
send test messages.

Cron runs the worker every five minutes to accumulate fresh matches; the worker
delivers daily subscribers at their chosen time, normally within one polling
interval. It is not a single server-time daily blast. The wrapper uses a shared
deployment lock and an exclusive notification lock, skips an active deployment,
and exits nonzero when prerequisites or the worker fail. The regular log rotation
already includes `logs/notifications.log`. Operators should check this log and
monitor nonzero runs; no external alerting service is configured by this PR.

## Acceptance and rollback

Use two controlled profiles with different skill/location criteria. Verify that
their saved searches differ, profile edits update criteria, closed/stale jobs
stay out, retry/repeated runs do not duplicate messages, and opted-out accounts
receive none. Check email links, unsubscribe, and the recorded notification
history. Observe two scheduled daily cycles before calling the feature shipped.
No-match days produce no digest, rather than an invented match or an empty email.

The worker examines the newest 200 hits per distinct query on each pass. Its
existing limits can miss a burst larger than that between passes; this delivery
does not promise exhaustive matching across such a burst. Delivered messages list
up to 10 jobs and link to the recorded results/live search for more.

To pause delivery without losing preferences or ledger/history:

```sh
rm -f /opt/hero-jobs/state/job-alerts.enabled
```

A run already in progress may finish; removing the marker prevents later runs.
No migrations, catalogue reindex, or deletion of user subscriptions is needed.
Live email acceptance remains pending email setup and controlled-recipient testing.

## Runner tests

On Linux with Python 3 and `flock`, run:

```sh
python3 -m unittest discover -s ops/hero-jobs -p test_notify_jobs.py -v
```

The tests invoke stub Docker/worker commands in temporary directories. They cover
the disabled default, missing configuration, deployment and notification locks,
concurrent launches, successful execution, and propagation of worker failures.
They do not connect to production or send messages.
