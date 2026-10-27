import importlib.util
import base64
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch
from urllib.parse import parse_qs, unquote, urlsplit


script = Path(__file__).resolve().parents[1] / "scripts" / "configure_worker_db.py"
spec = importlib.util.spec_from_file_location("configure_worker_db", script)
setup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(setup)


class FakeClient:
    def __init__(self, value):
        self.value = value

    def get_secret_value(self, **kwargs):
        return {"SecretString": json.dumps(self.value)}


class AccessDenied(Exception):
    response = {"Error": {"Code": "AccessDeniedException"}}


class ConfigureEnvironmentTests(unittest.TestCase):
    @unittest.skipUnless(shutil.which("terraform"), "Terraform is required for template rendering")
    def test_user_data_renders_the_helper_as_valid_python_and_shell(self):
        root = script.parents[1]
        values = {
            "aws_region": "ap-northeast-1", "queue_name": "test-queue",
            "bucket_name": "test-bucket", "repository_url": "https://example.invalid/repo.git",
            "repository_ref": "main", "worker_user": "judge-worker",
            "rust_toolchain": "stable", "isolate_git_ref": "v2.6",
            "db_host": "db.example.invalid", "db_port": 5432, "db_name": "judge",
            "db_secret_arn": "arn:aws:secretsmanager:ap-northeast-1:000000000000:secret:test",
            "worker_db_env_script": script.read_text(),
        }
        expression = (
            "base64encode(templatefile(" + json.dumps(str(root / "user_data.sh.tftpl"))
            + "," + json.dumps(values) + "))\n"
        )
        with tempfile.TemporaryDirectory() as temporary:
            result = subprocess.run(
                ["terraform", "console", "-no-color"], cwd=temporary,
                input=expression, text=True, capture_output=True,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            rendered = base64.b64decode(json.loads(result.stdout)).decode()
            result = subprocess.run(["bash", "-n"], input=rendered, text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            embedded = rendered.split("<<'WORKER_DB_PY'\n", 1)[1].split("\nWORKER_DB_PY", 1)[0]
            self.assertEqual(embedded.rstrip("\n"), script.read_text().rstrip("\n"))
            compile(embedded, "rendered-worker-db.py", "exec")

    def test_special_characters_round_trip_without_injecting_environment_lines(self):
        credentials = {"username": "worker name", "password": 'a:@%\\"$\n=秘密'}
        with tempfile.TemporaryDirectory() as temporary:
            destination = Path(temporary) / "config" / "worker.env"
            setup.configure_environment(
                FakeClient(credentials), "secret-arn", "db.example", 5432, "judge",
                destination,
            )
            content = destination.read_text()
            self.assertEqual(len(content.splitlines()), 1)
            url = urlsplit(content.strip().split("=", 1)[1])
            self.assertEqual(unquote(url.username), credentials["username"])
            self.assertEqual(unquote(url.password), credentials["password"])
            self.assertEqual(url.hostname, "db.example")
            self.assertEqual(parse_qs(url.query), {"sslmode": ["require"]})
            self.assertEqual(destination.stat().st_mode & 0o777, 0o600)
            self.assertEqual(destination.parent.stat().st_mode & 0o777, 0o700)

    def test_invalid_secret_preserves_existing_environment(self):
        for value in ({"username": "worker"}, {"username": "worker", "password": ""}, [], None):
            with self.subTest(value=value), tempfile.TemporaryDirectory() as temporary:
                destination = Path(temporary) / "worker.env"
                destination.write_text("existing configuration\n")
                with self.assertRaises(setup.SetupError):
                    setup.configure_environment(
                        FakeClient(value), "secret-arn", "db.example", 5432, "judge",
                        destination,
                    )
                self.assertEqual(destination.read_text(), "existing configuration\n")

    def test_atomic_replace_failure_preserves_existing_environment(self):
        with tempfile.TemporaryDirectory() as temporary:
            destination = Path(temporary) / "worker.env"
            destination.write_text("existing configuration\n")
            with patch.object(setup.os, "replace", side_effect=OSError("disk error")):
                with self.assertRaises(OSError):
                    setup.write_environment(destination, "replacement\n")
            self.assertEqual(destination.read_text(), "existing configuration\n")
            self.assertEqual(list(Path(temporary).iterdir()), [destination])

    def test_iam_propagation_failure_is_retried(self):
        class EventuallyAuthorized:
            calls = 0

            def get_secret_value(self, **kwargs):
                self.calls += 1
                if self.calls < 3:
                    raise AccessDenied("sensitive error text")
                return {"SecretString": '{"username":"worker","password":"value"}'}

        client = EventuallyAuthorized()
        delays = []
        result = setup.read_credentials(client, "secret-arn", sleep=delays.append)
        self.assertEqual(result["username"], "worker")
        self.assertEqual(delays, [1, 2])

    def test_permission_failure_is_bounded_and_does_not_expose_error_text(self):
        class NeverAuthorized:
            calls = 0

            def get_secret_value(self, **kwargs):
                self.calls += 1
                raise AccessDenied("sensitive error text")

        client = NeverAuthorized()
        with self.assertRaises(setup.SetupError) as error:
            setup.read_credentials(client, "secret-arn", sleep=lambda delay: None)
        self.assertEqual(client.calls, 8)
        self.assertNotIn("sensitive", str(error.exception))


if __name__ == "__main__":
    unittest.main()
