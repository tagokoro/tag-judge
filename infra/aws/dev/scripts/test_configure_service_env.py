import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock

from configure_service_env import (
    SetupError, environment_text, read_secret, write_environment,
)


class ServiceEnvironmentTests(unittest.TestCase):
    def test_complete_urls_are_preserved_for_each_service(self):
        for service, key in (("api", "DATABASE_URL"), ("worker", "JUDGE_DATABASE_URL")):
            url = "postgresql://user:p%40ss%2F%25@db.example.com:5432/judge?sslmode=require"
            with self.subTest(service=service):
                self.assertEqual(environment_text(service, {key: url}), f'{key}="{url}"\n')

    def test_only_approved_settings_are_exported(self):
        secret = {"DATABASE_URL": "url", "DEV_HEADER_AUTH": "true", "PATH": "/bad"}
        self.assertEqual(environment_text("api", secret), 'DATABASE_URL="url"\n')

    def test_rejects_old_format_missing_wrong_or_empty_url(self):
        for service in ("api", "worker"):
            key = "DATABASE_URL" if service == "api" else "JUDGE_DATABASE_URL"
            for secret in ({"username": "user", "password": "secret"}, {},
                           {key: " "}, {key: None}, {key: 123}):
                with self.subTest(service=service, secret=secret), self.assertRaises(SetupError):
                    environment_text(service, secret)
        with self.assertRaises(SetupError):
            environment_text("worker", {"DATABASE_URL": "url"})

    def test_rejects_injection_and_partial_oauth(self):
        for extra in ({"DATABASE_URL": "url\nDEV_HEADER_AUTH=true"},
                      {"GOOGLE_OAUTH_CLIENT_ID": "client"},
                      {"AUTH_SUCCESS_REDIRECT_URL": "/\r\nPATH=/bad"}):
            with self.subTest(extra=extra), self.assertRaises(SetupError):
                environment_text("api", {"DATABASE_URL": "url", **extra})

    def test_oauth_values_are_escaped_for_systemd(self):
        secret = {"DATABASE_URL": "url", "GOOGLE_OAUTH_CLIENT_ID": "client",
                  "GOOGLE_OAUTH_CLIENT_SECRET": 'quote"slash\\dollar$',
                  "GOOGLE_OAUTH_REDIRECT_URL": "http://localhost/callback"}
        self.assertIn('GOOGLE_OAUTH_CLIENT_SECRET="quote\\"slash\\\\dollar$"\n',
                      environment_text("api", secret))

    def test_reads_json_and_retries_access_denied_without_leaking_secrets(self):
        denied = Exception("sensitive error")
        denied.response = {"Error": {"Code": "AccessDeniedException"}}
        client = Mock()
        client.get_secret_value.side_effect = [denied, {"SecretString": json.dumps({"DATABASE_URL": "url"})}]
        sleep = Mock()
        self.assertEqual(read_secret(client, "secret-arn", sleep), {"DATABASE_URL": "url"})
        sleep.assert_called_once_with(1)
        client.get_secret_value.side_effect = Exception("sensitive error")
        with self.assertRaises(SetupError) as error:
            read_secret(client, "secret-arn", sleep)
        self.assertNotIn("sensitive", str(error.exception))

    def test_rejects_invalid_secret_payloads(self):
        client = Mock()
        for payload in ({}, {"SecretString": "bad-secret-data"}, {"SecretString": "[]"}):
            client.get_secret_value.return_value = payload
            with self.subTest(payload=payload), self.assertRaises(SetupError):
                read_secret(client, "secret-arn")

    def test_atomic_file_is_private_and_replaced(self):
        with tempfile.TemporaryDirectory() as temporary:
            destination = Path(temporary) / "config" / "api.env"
            write_environment(destination, "old")
            write_environment(destination, "new")
            self.assertEqual(destination.read_text(), "new")
            self.assertEqual(destination.stat().st_mode & 0o777, 0o600)
            self.assertEqual(destination.parent.stat().st_mode & 0o777, 0o700)
            self.assertEqual(list(destination.parent.iterdir()), [destination])


if __name__ == "__main__":
    unittest.main()
