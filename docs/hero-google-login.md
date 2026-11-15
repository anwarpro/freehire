# Google sign-in for PH Jobs

Google sign-in already uses the existing OAuth provider, verified-email account
linking, cookie session, and provider-driven login buttons. This delivery supplies
the deployment setup and readiness check. It adds no location collection or new
profile/onboarding requirement. Live sign-in remains pending real credentials and
a successful browser callback.

## Configure the Google client

Use the school's Google Cloud project and create an OAuth **Web application**
client. Configure the consent screen/audience for the intended student and mentor
users; if the application is in testing, add the intended test accounts. Register
this exact authorized redirect URI:

```text
https://jobs.programming-hero.org/api/v1/auth/oauth/google/callback
```

The origin is `https://jobs.programming-hero.org`. The backend authorization-code
flow uses the redirect URI above; no browser Google SDK, Maps, device geolocation,
Drive, Calendar, or extra data scopes are needed. The provider requests only
`openid` and `email`. The callback URI must match exactly, including scheme/path.
See [Google's web-server OAuth setup](https://developers.google.com/identity/protocols/oauth2/web-server#creatingcred).

## Supply credentials securely

On the server, edit `/opt/hero-jobs/.env` with a protected editor and add the two
entries shown in [the unpopulated example](../ops/hero-jobs/google-login.env.example).
Keep the existing file and its other values; do not replace it with the example.
Keep permissions at `600`. Do not paste secrets into chat, PRs, command arguments,
logs, or frontend environment variables.

The app already loads this file via Compose `env_file`. The existing Compose
configuration sets `FRONTEND_ORIGIN=https://jobs.programming-hero.org` and
`COOKIE_SECURE=true`. No additional provider implementation or frontend rebuild
is needed. After saving the values, an operator must recreate the app container
to reload them (a restart alone does not reload Compose environment):

```sh
cd /opt/hero-jobs
docker compose up -d --no-deps --force-recreate app
```

This causes a brief backend restart. Verify HTTPS health before testing sign-in.
To disable Google again, remove only these two variables and recreate app; other
login methods and stored accounts stay available.

## Verify

```sh
python3 /opt/freehire/ops/hero-jobs/check-google-login.py
```

This read-only check calls the public provider list and sign-in start endpoint.
It does not follow the Google redirect, send credentials, log in, or create an
account. It checks the callback, sign-in-only scopes, authorization code flow,
and Secure/HttpOnly state cookie without printing sensitive response values.
Passing readiness does not prove that Google accepts the client credentials.

Then use the deployed Sign in dialog and its Google button. Verify a new test
account, a returning account, safe linking of an existing verified-email account,
return to the requested job page, cancellation/error behavior, and logout. Staff
access must remain explicitly assigned; Google sign-in grants no reviewer/admin
role. Record successful browser evidence and deployment date before marking the
feature shipped. Complete this with controlled accounts; do not change real users
or their credentials to manufacture test evidence.
