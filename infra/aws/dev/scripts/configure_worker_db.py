#!/usr/bin/python3
"""Restore the worker's DB environment from Secrets Manager during EC2 setup."""

import argparse
import json
import os
from pathlib import Path
import sys
import tempfile
import time
from urllib.parse import quote


class SetupError(Exception):
    """An error message that is safe to include in cloud-init logs."""


def read_credentials(client, secret_arn, sleep=time.sleep):
    # A newly attached instance policy may not be effective immediately.
    for attempt in range(8):
        try:
            response = client.get_secret_value(SecretId=secret_arn)
            break
        except Exception as error:
            code = getattr(error, "response", {}).get("Error", {}).get("Code", "")
            if code in ("AccessDeniedException", "AccessDenied") and attempt < 7:
                sleep(min(2**attempt, 8))
                continue
            # Do not include exception text or the SecretString in logs.
            raise SetupError("Could not retrieve the worker DB secret") from None

    try:
        credentials = json.loads(response["SecretString"])
    except (KeyError, TypeError, ValueError):
        raise SetupError("Worker DB secret must be a JSON SecretString") from None
    if not isinstance(credentials, dict) or any(
        not isinstance(credentials.get(key), str) or not credentials[key]
        for key in ("username", "password")
    ):
        raise SetupError("Worker DB secret needs nonempty username and password strings")
    return credentials


def environment_text(host, port, database, credentials):
    if not host or any(character.isspace() or character in "/?#@:" for character in host):
        raise SetupError("Invalid RDS hostname")
    if not 1 <= port <= 65535 or not database:
        raise SetupError("Invalid RDS port or database name")
    username = quote(credentials["username"], safe="")
    password = quote(credentials["password"], safe="")
    name = quote(database, safe="")
    # Native TLS validates the RDS certificate against the system trust store.
    return (
        f"JUDGE_DATABASE_URL=postgresql://{username}:{password}@{host}:{port}/{name}"
        "?sslmode=require\n"
    )


def write_environment(destination, content):
    destination = Path(destination)
    destination.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    destination.parent.chmod(0o700)
    descriptor, temporary = tempfile.mkstemp(prefix=".worker-env-", dir=destination.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="ascii") as output:
            os.fchmod(output.fileno(), 0o600)
            output.write(content)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, destination)
    finally:
        Path(temporary).unlink(missing_ok=True)


def configure_environment(client, secret_arn, host, port, database, destination):
    credentials = read_credentials(client, secret_arn)
    write_environment(destination, environment_text(host, port, database, credentials))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--region", required=True)
    parser.add_argument("--secret-arn", required=True)
    parser.add_argument("--host", required=True)
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--database", required=True)
    args = parser.parse_args()
    try:
        import boto3
        from botocore.config import Config

        client = boto3.client(
            "secretsmanager",
            region_name=args.region,
            config=Config(
                connect_timeout=5,
                read_timeout=10,
                retries={"mode": "standard", "total_max_attempts": 3},
            ),
        )
        configure_environment(
            client, args.secret_arn, args.host, args.port, args.database,
            "/etc/tag-judge/worker.env",
        )
    except SetupError as error:
        print(str(error), file=sys.stderr)
        return 1
    except Exception:
        print("Could not configure the worker DB environment", file=sys.stderr)
        return 1
    print("Worker DB environment configured")
    return 0


if __name__ == "__main__":
    sys.exit(main())
