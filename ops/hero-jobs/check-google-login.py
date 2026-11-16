#!/usr/bin/env python3
"""Read-only Google sign-in readiness check; never follows the Google redirect."""
import json
import sys
from http.cookies import SimpleCookie
from urllib.error import HTTPError, URLError
from urllib.parse import parse_qs, urlparse
from urllib.request import HTTPRedirectHandler, build_opener

ORIGIN = "https://jobs.programming-hero.org"


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def check(opener):
    with opener.open(ORIGIN + "/api/v1/auth/oauth/providers", timeout=15) as response:
        providers = json.load(response).get("data", [])
    if "google" not in providers:
        raise ValueError("Google is disabled. Configure both OAUTH_GOOGLE_CLIENT_ID and OAUTH_GOOGLE_CLIENT_SECRET in the protected server environment, then recreate app.")
    try:
        opener.open(ORIGIN + "/api/v1/auth/oauth/google/start?returnTo=%2Fjobs", timeout=15)
    except HTTPError as response:
        if response.code != 302:
            raise ValueError("Google start did not return HTTP 302.") from None
        target = urlparse(response.headers.get("Location", ""))
        query = parse_qs(target.query)
        if target.scheme != "https" or target.hostname != "accounts.google.com":
            raise ValueError("Google start returned an unexpected authorization host.")
        expected = ORIGIN + "/api/v1/auth/oauth/google/callback"
        if query.get("redirect_uri") != [expected] or query.get("response_type") != ["code"]:
            raise ValueError("The callback URL or authorization-code response type is incorrect.")
        if not query.get("client_id") or not query.get("state"):
            raise ValueError("The client ID or OAuth state is missing.")
        if set(query.get("scope", [""])[0].split()) != {"openid", "email"}:
            raise ValueError("Expected sign-in-only scopes: openid and email.")
        cookies = SimpleCookie()
        for header in response.headers.get_all("Set-Cookie", []):
            cookies.load(header)
        state = cookies.get("hire_oauth_state")
        if state is None or not state["secure"] or not state["httponly"]:
            raise ValueError("The OAuth state cookie must be Secure and HttpOnly.")
        if state.value != query["state"][0]:
            raise ValueError("The redirect and cookie OAuth states do not match.")
        return
    raise ValueError("Google start did not redirect.")


def main():
    try:
        check(build_opener(NoRedirect()))
    except (ValueError, URLError, OSError, json.JSONDecodeError) as error:
        # Do not print response bodies, authorization URLs, cookies, or credentials.
        if isinstance(error, ValueError):
            print(str(error), file=sys.stderr)
        else:
            print("Could not verify Google login over HTTPS. Check service health and try again.", file=sys.stderr)
        return 1
    print("Google provider, callback, scopes and state-cookie checks passed. Complete a real browser sign-in before marking login verified.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
