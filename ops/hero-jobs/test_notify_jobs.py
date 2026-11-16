"""Linux runner contracts. Docker and the notification binary are disposable stubs."""
import fcntl
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
import unittest


@unittest.skipUnless(shutil.which("flock"), "requires Linux flock")
class NotifyRunnerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="ph alerts ")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / "state").mkdir()
        (self.root / "bin").mkdir()
        self.runner = self.root / "notify-jobs.sh"
        shutil.copyfile(Path(__file__).with_name("notify-jobs.sh"), self.runner)
        self.log = self.root / "calls.jsonl"
        self.worker = self.root / "worker.py"
        self.worker.write_text('''#!/usr/bin/env python3
import os, pathlib, sys, time
pathlib.Path(os.environ['RUNNER_TEST_STARTED']).touch()
if os.environ.get('RUNNER_TEST_WAIT'):
    deadline = time.monotonic() + 5
    while not pathlib.Path(os.environ['RUNNER_TEST_RELEASE']).exists():
        if time.monotonic() > deadline: sys.exit(99)
        time.sleep(0.02)
sys.exit(int(os.environ.get('RUNNER_TEST_EXIT', '0')))
''')
        self.worker.chmod(0o700)
        docker = self.root / "bin" / "docker"
        docker.write_text('''#!/usr/bin/env python3
import json, os, shlex, subprocess, sys
assert sys.argv[1:-1] == ['compose', 'run', '--rm', '-T', '--no-deps', '--entrypoint', '/bin/sh', 'tools', '-c']
with open(os.environ['RUNNER_TEST_LOG'], 'a') as f: f.write(json.dumps(sys.argv[1:]) + '\\n')
assert 'exec /app/notify' in sys.argv[-1]
command = sys.argv[-1].replace('exec /app/notify', 'exec ' + shlex.quote(os.environ['RUNNER_TEST_WORKER']))
sys.exit(subprocess.run(['/bin/sh', '-c', command]).returncode)
''')
        docker.chmod(0o700)
        self.env = {**os.environ, "PATH": str(self.root / "bin") + os.pathsep + os.environ["PATH"],
                    "AWS_REGION": "test-region", "NOTIFY_EMAIL_FROM": "test@example.invalid",
                    "MEILI_MASTER_KEY": "test-key", "JWT_SECRET": "test-secret",
                    "RUNNER_TEST_LOG": str(self.log), "RUNNER_TEST_WORKER": str(self.worker),
                    "RUNNER_TEST_STARTED": str(self.root / "started"),
                    "RUNNER_TEST_RELEASE": str(self.root / "release")}

    def enable(self):
        (self.root / "state" / "job-alerts.enabled").touch()

    def run_runner(self):
        return subprocess.run(["sh", str(self.runner)], env=self.env, capture_output=True, text=True, timeout=10)

    def calls(self):
        return [json.loads(line) for line in self.log.read_text().splitlines()] if self.log.exists() else []

    def test_disabled_runner_never_invokes_docker(self):
        self.assertEqual(self.run_runner().returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_deployment_marker_skips_delivery(self):
        self.enable()
        (self.root / "state" / "deploying").touch()
        self.assertEqual(self.run_runner().returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_real_deployment_and_notification_locks_skip_delivery(self):
        self.enable()
        for name in ["deploy.lock", "notify.lock"]:
            with self.subTest(lock=name), (self.root / "state" / name).open("w") as lock:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
                self.assertEqual(self.run_runner().returncode, 0)
                self.assertEqual(self.calls(), [])

    def test_missing_prerequisites_fail_before_worker(self):
        self.enable()
        for key in ["AWS_REGION", "NOTIFY_EMAIL_FROM", "MEILI_MASTER_KEY", "JWT_SECRET"]:
            with self.subTest(key=key):
                value = self.env.pop(key)
                result = self.run_runner()
                self.env[key] = value
                self.assertEqual(result.returncode, 1)
                self.assertIn("configure email", result.stderr)
                self.assertFalse((self.root / "started").exists())

    def test_configured_runner_invokes_existing_worker(self):
        self.enable()
        self.assertEqual(self.run_runner().returncode, 0)
        self.assertEqual(len(self.calls()), 1)
        self.assertTrue((self.root / "started").exists())

    def test_worker_failure_reaches_cron(self):
        self.enable()
        self.env["RUNNER_TEST_EXIT"] = "7"
        self.assertEqual(self.run_runner().returncode, 7)

    def test_concurrent_runner_does_not_deliver_twice(self):
        self.enable()
        self.env["RUNNER_TEST_WAIT"] = "1"
        first = subprocess.Popen(["sh", str(self.runner)], env=self.env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            deadline = time.monotonic() + 3
            while not (self.root / "started").exists():
                self.assertLess(time.monotonic(), deadline, "first runner did not reach worker")
                time.sleep(0.02)
            self.assertEqual(self.run_runner().returncode, 0)
            self.assertEqual(len(self.calls()), 1)
        finally:
            (self.root / "release").touch()
            first.communicate(timeout=5)
        self.assertEqual(first.returncode, 0)


if __name__ == "__main__":
    unittest.main()
