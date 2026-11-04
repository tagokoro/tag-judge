#!/usr/bin/python3
"""Copy approved settings from a JSON SecretString into a systemd environment file."""

import argparse
import json
import os
from pathlib import Path
import sys
import tempfile
import time


class SetupError(Exception):
    """A setup error whose message does not contain secret values."""


def read_secret(client, secret_arn, sleep=time.sleep):
    for attempt in range(8):
        try:
            response = client.get_secret_value(SecretId=secret_arn)
            break
        except Exception as error:
            code = getattr(error, "response", {}).get("Error", {}).get("Code", "")
            if code in ("AccessDeniedException", "AccessDenied") and attempt < 7:
                sleep(min(2**attempt, 8))
                continue
            raise SetupError("Could not retrieve the service secret") from None
    try:
        secret = json.loads(response["SecretString"])
    except (KeyError, TypeError, ValueError):
        raise SetupError("Secret must be a JSON SecretString") from None
    if not isinstance(secret, dict):
        raise SetupError("Secret must contain a JSON object")
    return secret


def environment_text(service, secret):
    if service not in ("api", "worker"):
        raise SetupError("Unknown service")
    database_key = "DATABASE_URL" if service == "api" else "JUDGE_DATABASE_URL"
    if not isinstance(secret.get(database_key), str) or not secret[database_key].strip():
        raise SetupError("Secret is missing the service database URL")
    values = {database_key: secret[database_key]}
    if service == "api":
        oauth_keys = (
            "GOOGLE_OAUTH_CLIENT_ID", "GOOGLE_OAUTH_CLIENT_SECRET",
            "GOOGLE_OAUTH_REDIRECT_URL",
        )
        if any(secret.get(key) for key in oauth_keys) and not all(
            secret.get(key) for key in oauth_keys
        ):
            raise SetupError("All three Google OAuth settings must be supplied together")
        for key in (*oauth_keys, "AUTH_SUCCESS_REDIRECT_URL"):
            if key in secret:
                values[key] = secret[key]
    lines = []
    for key, value in values.items():
        if not isinstance(value, str) or any(ord(c) < 32 or ord(c) > 126 for c in value):
            raise SetupError("Settings must contain printable ASCII strings")
        # Preserve values literally in systemd's double-quoted EnvironmentFile format.
        escaped = value.replace("\\", "\\\\").replace('"', '\\"')
        lines.append(f'{key}="{escaped}"\n')
    return "".join(lines)


def write_environment(destination, content):
    destination = Path(destination)
    destination.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    destination.parent.chmod(0o700)
    descriptor, temporary = tempfile.mkstemp(prefix=".service-env-", dir=destination.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="ascii") as output:
            os.fchmod(output.fileno(), 0o600)
            output.write(content)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, destination)
    finally:
        Path(temporary).unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--region", required=True)
    parser.add_argument("--secret-arn", required=True)
    parser.add_argument("--service", choices=("api", "worker"), required=True)
    args = parser.parse_args()
    try:
        import boto3
        from botocore.config import Config

        client = boto3.client("secretsmanager", region_name=args.region, config=Config(
            connect_timeout=5, read_timeout=10,
            retries={"mode": "standard", "total_max_attempts": 3},
        ))
        secret = read_secret(client, args.secret_arn)
        write_environment(
            f"/etc/tag-judge/{args.service}.env", environment_text(args.service, secret),
        )
    except SetupError as error:
        print(str(error), file=sys.stderr)
        return 1
    except Exception:
        print("Could not configure the service environment", file=sys.stderr)
        return 1
    print("Service environment configured")
    return 0


if __name__ == "__main__":
    sys.exit(main())
