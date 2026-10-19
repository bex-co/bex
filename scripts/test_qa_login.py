#!/usr/bin/env python3
"""Exercise the password-isolated QA helper with a scripted curl replacement."""

import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import tempfile
import unittest
from urllib.parse import urlencode


SCRIPT = Path(__file__).with_name("qa-login.sh")
AUTH = "https://auth.example.test"
DASH = "https://dashboard.example.test"
OAUTH = "https://oauth.example.test"
EMAIL = "qa@example.test"
PASSWORD = 'test-only-password-$()"\\special'
CSRF = "test-csrf"
ACTION = AUTH + "/self-service/login?flow=test-flow"
SUCCESS = DASH + "/auth/device/success"
COOKIE = "#HttpOnly_auth.example.test\tFALSE\t/\tTRUE\t0\tory_kratos_session\ttest-session\n"

MOCK_CURL = r'''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
args = sys.argv[1:]
options, headers, url = {}, [], None
i = 0
while i < len(args):
    arg = args[i]
    if arg in ("-o", "-D", "-w", "-c", "-b", "--data-binary", "-H", "--connect-timeout", "--max-time"):
        value = args[i + 1]
        if arg == "-H":
            headers.append(value)
        else:
            options[arg] = value
        i += 2
    elif arg.startswith("-"):
        assert arg in ("-sS", "-fsS", "-L"), "unexpected curl flag"
        i += 1
    else:
        assert url is None, "multiple URLs"
        url = arg
        i += 1
assert options.get("--connect-timeout") == "10", "request has no bounded connection timeout"
assert options.get("--max-time") == "30", "request has no bounded total timeout"
log = Path(os.environ["MOCK_LOG"])
calls = json.loads(log.read_text()) if log.exists() else []
steps = json.loads(Path(os.environ["MOCK_STEPS"]).read_text())
if len(calls) >= len(steps):
    calls.append({"unexpected_request": True})
    log.write_text(json.dumps(calls))
assert len(calls) < len(steps), "unexpected request"
step = steps[len(calls)]
assert url == step["url"], "unexpected request URL"
assert headers == step.get("headers", []), "unexpected request headers"
if "required_cookie" in step:
    assert step["required_cookie"] in Path(options["-b"]).read_text(), "seed cookie missing"
if step.get("password"):
    assert "-L" not in args, "password submission must not follow redirects"
    assert "--data-binary" in options, "missing password body"
    assert options["--data-binary"].startswith("@"), "body must be passed by file"
    body = json.loads(Path(options["--data-binary"][1:]).read_text())
    assert body == {"method": "password", "identifier": os.environ["QA_EMAIL"],
                    "password": os.environ["QA_PASSWORD"], "csrf_token": "test-csrf"}, "wrong body"
else:
    assert "--data-binary" not in options, "redirect forwarded password body"
assert os.environ["QA_PASSWORD"] not in " ".join(args), "password in argv"
assert os.environ["QA_EMAIL"] not in " ".join(args), "email in argv"
calls.append({"url": url, "argv": args})
log.write_text(json.dumps(calls))
if "-c" in options:
    jar = Path(options["-c"])
    previous = jar.read_text() if jar.exists() else "# Netscape HTTP Cookie File\n"
    jar.write_text(previous + step.get("cookie", ""))
code = step.get("status", 200)
if "-D" in options:
    text = "HTTP/1.1 %s Test\r\n" % code
    if "location" in step:
        text += "Location: %s\r\n" % step["location"]
    Path(options["-D"]).write_text(text + "\r\n")
body = step.get("body", {})
body = body if isinstance(body, str) else json.dumps(body)
if "-o" in options:
    if options["-o"] != "/dev/null":
        Path(options["-o"]).write_text(body)
else:
    sys.stdout.write(body)
if "-w" in options:
    assert options["-w"] == "%{http_code}"
    sys.stdout.write(str(code))
if "-fsS" in args and code >= 400:
    sys.exit(22)
'''


def challenge_url(challenge="test"):
    return AUTH + "/self-service/login/browser?" + urlencode({
        "return_to": DASH + "/", "login_challenge": challenge})


def flow(url=None, **extra):
    return {
        "url": url or AUTH + "/self-service/login/browser?return_to=" + DASH + "/",
        "headers": ["Accept: application/json"],
        "body": {"ui": {"action": ACTION, "nodes": [
            {"attributes": {"name": "csrf_token", "value": CSRF}}
        ]}, "refresh": True},
        **extra,
    }


def submit(**extra):
    return {"url": ACTION, "password": True, "cookie": COOKIE,
            "headers": ["Accept: application/json", "Content-Type: application/json"], **extra}


def whoami(status=200):
    return {"url": AUTH + "/sessions/whoami", "status": status}


def revoke_session():
    return [whoami(),
            {"url": AUTH + "/self-service/logout/browser?return_to=" + DASH + "/",
             "headers": ["Accept: application/json"],
             "body": {"logout_url": AUTH + "/self-service/logout?token=test"}},
            {"url": AUTH + "/self-service/logout?token=test"}, whoami(401)]


class QALoginTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / "scripts").mkdir()
        (self.root / "bin").mkdir()
        shutil.copyfile(SCRIPT, self.root / "scripts/qa-login.sh")
        self.mock = self.root / "bin/curl"
        self.mock.write_text(MOCK_CURL)
        self.mock.chmod(0o755)
        self.env = {**os.environ, "PATH": str(self.mock.parent) + os.pathsep + os.environ["PATH"],
                    "QA_EMAIL": EMAIL, "QA_PASSWORD": PASSWORD,
                    "KRATOS_PUB": AUTH, "DASH": DASH, "OAUTH_PUB": OAUTH,
                    "MOCK_STEPS": str(self.root / "steps.json"),
                    "MOCK_LOG": str(self.root / "calls.json")}

    def run_login(self, steps, *args, ok=True, cleanup=False):
        if cleanup:
            steps = steps + revoke_session()
        (self.root / "steps.json").write_text(json.dumps(steps))
        result = subprocess.run(["bash", str(self.root / "scripts/qa-login.sh"), *args],
                                env=self.env, text=True, capture_output=True, timeout=20)
        self.assertNotIn(PASSWORD, result.stdout + result.stderr)
        self.assertNotIn(EMAIL, result.stdout + result.stderr)
        self.assertEqual(result.returncode == 0, ok, result.stderr)
        log = self.root / "calls.json"
        self.assertEqual(len(json.loads(log.read_text())) if log.exists() else 0, len(steps), result.stderr)
        if not ok:
            self.assertEqual(result.stdout, "")
            self.assertEqual(list(self.root.glob(".playwright-mcp/qa-session-*.jar")), [])
        return result

    def handles(self, result):
        """The (state, jar) paths a successful login printed on its two lines."""
        ok, logout = result.stdout.splitlines()
        self.assertTrue(ok.startswith("ok ") and logout.startswith("logout "), result.stdout)
        return Path(ok[3:]), Path(logout[len("logout "):])

    def assert_state(self, result, session="test-session"):
        state, jar = self.handles(result)
        cookies = json.loads(state.read_text())["cookies"]
        self.assertEqual(next(c["value"] for c in cookies if c["name"] == "ory_kratos_session"), session)
        self.assertRegex(jar.name, r"^qa-session-[0-9a-f]{12}\.jar$")
        for path in (state, jar):
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
        return state, jar

    def test_ordinary_login_preserves_state_and_jar(self):
        result = self.run_login([flow(), submit(), whoami()], "custom/state.json")
        state, _ = self.assert_state(result)
        self.assertEqual(state, self.root / "custom/state.json")

    def test_concurrent_logins_keep_separate_logout_handles(self):
        # w4/343: two hunts in one checkout shared one jar, so one run's
        # --logout revoked the other's session. Each login now gets its own.
        first = self.assert_state(self.run_login([flow(), submit(), whoami()]))
        snapshot = [path.read_bytes() for path in first]
        (self.root / "calls.json").unlink()  # the mock replays steps by call count
        other = submit(cookie=COOKIE.replace("test-session", "other-session"))
        second = self.assert_state(self.run_login([flow(), other, whoami()]), session="other-session")
        self.assertTrue(set(first).isdisjoint(second), (first, second))
        self.assertEqual([path.read_bytes() for path in first], snapshot)

    def test_logout_requires_the_printed_handle(self):
        # A jar at the old shared path is someone else's session: a bare
        # --logout must refuse, not revoke whatever it finds there.
        shared = self.root / ".playwright-mcp/qa-session.jar"
        shared.parent.mkdir(parents=True, exist_ok=True)
        shared.write_text("# Netscape HTTP Cookie File\n" + COOKIE)
        result = self.run_login([], "--logout", ok=False)
        self.assertEqual(result.returncode, 2)
        self.assertIn("requires the path", result.stderr)

    def test_challenge_is_encoded_and_json_redirects_finish_consent(self):
        challenge = "opaque/+&? #=%challenge"
        bootstrap = challenge_url(challenge)
        success = SUCCESS + "?client_id=429024F5E608930E2A65EF92591A25CC"
        result = self.run_login([
            flow(bootstrap),
            submit(status=422, body={"redirect_browser_to": OAUTH + "/oauth2/auth?login_verifier=v"}),
            {"url": OAUTH + "/oauth2/auth?login_verifier=v", "status": 302,
             "location": "/oauth2/continue"},
            {"url": OAUTH + "/oauth2/continue", "status": 303,
             "location": DASH + "/auth/consent?consent_challenge=c"},
            {"url": DASH + "/auth/consent?consent_challenge=c", "status": 303,
             "location": OAUTH + "/oauth2/device/verify?consent_verifier=c"},
            {"url": OAUTH + "/oauth2/device/verify?consent_verifier=c", "status": 302,
             "location": success},
            {"url": success, "body": "<html>Authorized</html>"}, whoami(),
        ], "--login-challenge", challenge)
        self.assert_state(result)

    def test_post_307_becomes_get_without_forwarding_credentials(self):
        bootstrap = challenge_url()
        self.run_login([flow(bootstrap), submit(status=307, location=SUCCESS),
                        {"url": SUCCESS}, whoami()], "--login-challenge", "test")

    def test_bootstrap_can_redirect_without_a_password_form(self):
        bootstrap = challenge_url()
        self.run_login([flow(bootstrap, status=422, body={"redirect_browser_to": SUCCESS}, cookie=COOKIE),
                        {"url": SUCCESS}, whoami()], "--login-challenge", "test")

    def test_null_bootstrap_reasks_as_browser(self):
        bootstrap = challenge_url()
        self.run_login([
            flow(bootstrap, body="null", cookie=COOKIE),
            {"url": bootstrap, "status": 303, "location": SUCCESS},
            {"url": SUCCESS}, whoami(),
        ], "--login-challenge", "test")

    def test_seed_jar_preserves_hydra_cookies_without_modifying_input(self):
        bootstrap = challenge_url()
        seed = self.root / "device.jar"
        original = "# Netscape HTTP Cookie File\n" + (
            "oauth.example.test\tFALSE\t/\tTRUE\t0\toauth_csrf\ttest-hydra-csrf\n")
        seed.write_text(original)
        result = self.run_login([
            flow(bootstrap, required_cookie="test-hydra-csrf"),
            submit(status=422, body={"redirect_browser_to": SUCCESS}),
            {"url": SUCCESS, "required_cookie": "test-hydra-csrf"}, whoami(),
        ], "--login-challenge", "test", "--cookie-jar", str(seed))
        self.assertEqual(seed.read_text(), original)
        _, jar = self.assert_state(result)
        self.assertIn("test-hydra-csrf", jar.read_text())

    def test_continue_with_redirect(self):
        bootstrap = challenge_url()
        self.run_login([flow(bootstrap), submit(body={"continue_with": [
            {"action": "redirect_browser_to", "redirect_browser_to": SUCCESS}]}),
            {"url": SUCCESS}, whoami()], "--login-challenge", "test")

    def test_untrusted_form_action_is_refused_before_password_submission(self):
        response = flow()
        response["body"]["ui"]["action"] = "https://attacker.example.test/login"
        result = self.run_login([response], ok=False)
        self.assertIn("refusing an untrusted login action", result.stderr)

    def test_untrusted_redirects_are_refused(self):
        bootstrap = challenge_url()
        for target in ("https://attacker.example.test/", "file:///tmp/secret",
                       "http://oauth.example.test/", "https://oauth.example.test:8443/",
                       "https://user:password@oauth.example.test/"):
            with self.subTest(target=target):
                (self.root / "calls.json").unlink(missing_ok=True)
                result = self.run_login([flow(bootstrap), submit(status=303, location=target)],
                                        "--login-challenge", "test", ok=False, cleanup=True)
                self.assertIn("refusing an untrusted login redirect", result.stderr)

    def test_refused_challenge_never_falls_back_to_ordinary_login(self):
        bootstrap = challenge_url("expired")
        self.run_login([flow(bootstrap, status=400, body={"error": {"reason": PASSWORD}})],
                       "--login-challenge", "expired", ok=False)

    def test_consent_html_is_not_mistaken_for_completed_authorization(self):
        bootstrap = challenge_url()
        consent = DASH + "/auth/consent?consent_challenge=c"
        result = self.run_login([flow(bootstrap), submit(status=303, location=consent),
                        {"url": consent, "body": "<html>Allow this app?</html>"}],
                       "--login-challenge", "test", ok=False, cleanup=True)
        self.assertIn("endpoint dashboard /auth/consent; query keys: consent_challenge", result.stderr)
        self.assertNotIn("consent_challenge=c", result.stderr)
        self.assertIn("revoked the temporary QA session", result.stderr)

    def test_failed_challenge_never_revokes_a_seeded_session(self):
        bootstrap = challenge_url()
        seed = self.root / "existing.jar"
        seed.write_text(COOKIE)
        result = self.run_login([flow(bootstrap), submit(status=303, location=DASH + "/auth/login"),
                                {"url": DASH + "/auth/login", "body": "<html>Login</html>"}],
                               "--cookie-jar", str(seed), "--login-challenge", "test", ok=False)
        self.assertNotIn("temporary QA session", result.stderr)
        self.assertEqual(seed.read_text(), COOKIE)

    def test_failed_success_landing_and_redirect_loops_are_refused(self):
        bootstrap = challenge_url()
        self.run_login([flow(bootstrap), submit(status=303, location=SUCCESS),
                        {"url": SUCCESS, "status": 403, "body": PASSWORD}],
                       "--login-challenge", "test", ok=False, cleanup=True)
        (self.root / "calls.json").unlink()
        loop = OAUTH + "/loop"
        result = self.run_login([flow(bootstrap), submit(status=303, location=loop)] +
                                [{"url": loop, "status": 302, "location": loop}] * 20,
                                "--login-challenge", "test", ok=False, cleanup=True)
        self.assertIn("exceeded the redirect limit", result.stderr)

    def test_success_landing_with_error_query_is_refused_without_query_values(self):
        bootstrap = challenge_url()
        for query in ("error=private-error-value", "client_id=private-error-value", "client_id=",
                      "client_id=429024F5E608930E2A65EF92591A25CC&error=private-error-value",
                      "client_id=429024F5E608930E2A65EF92591A25CC&client_id=private-error-value"):
            with self.subTest(query=query):
                (self.root / "calls.json").unlink(missing_ok=True)
                target = SUCCESS + "?" + query
                result = self.run_login([flow(bootstrap), submit(status=303, location=target), {"url": target}],
                                        "--login-challenge", "test", ok=False, cleanup=True)
                self.assertIn("endpoint dashboard /auth/device/success; query keys:", result.stderr)
                self.assertNotIn("private-error-value", result.stderr)

    def test_failed_login_does_not_print_server_echo_of_credentials(self):
        self.run_login([flow(), submit(status=400, body={"ui": {"messages": [{"text": PASSWORD}]}}),
                        whoami(401)], ok=False)

    def test_invalid_challenge_options_do_not_contact_kratos(self):
        for args in (("--login-challenge",), ("--login-challenge", ""),
                     ("--cookie-jar",), ("--cookie-jar", ""),
                     ("--login-challenge", "test", "--login-challenge", "again"),
                     ("--login-challenge", "test", "--logout"),
                     ("--logout", "--login-challenge", "test"),
                     ("--cookie-jar", "seed.jar", "--logout")):
            with self.subTest(args=args):
                self.run_login([], *args, ok=False)

    def test_logout_still_revokes_storage_state(self):
        state = self.root / "state.json"
        state.write_text(json.dumps({"cookies": [{"domain": "auth.example.test", "path": "/",
                         "httpOnly": True, "name": "ory_kratos_session", "value": "test-session"}]}))
        result = self.run_login(revoke_session(), "--logout", str(state))
        self.assertEqual(result.stdout, "ok logged-out\n")


if __name__ == "__main__":
    unittest.main()
