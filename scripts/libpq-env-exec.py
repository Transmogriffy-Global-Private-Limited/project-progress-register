#!/usr/bin/env python3
"""Execute a libpq client using the configured PostgreSQL URL via PG* variables."""

from __future__ import annotations

import os
import sys
from urllib.parse import parse_qsl, unquote, urlsplit


LIBPQ_ENV = {
    "application_name": "PGAPPNAME",
    "channel_binding": "PGCHANNELBINDING",
    "connect_timeout": "PGCONNECT_TIMEOUT",
    "dbname": "PGDATABASE",
    "gssencmode": "PGGSSENCMODE",
    "gsslib": "PGGSSLIB",
    "host": "PGHOST",
    "hostaddr": "PGHOSTADDR",
    "keepalives": "PGKEEPALIVES",
    "keepalives_count": "PGKEEPALIVESCOUNT",
    "keepalives_idle": "PGKEEPALIVESIDLE",
    "keepalives_interval": "PGKEEPALIVESINTERVAL",
    "krbsrvname": "PGKRBSRVNAME",
    "options": "PGOPTIONS",
    "passfile": "PGPASSFILE",
    "password": "PGPASSWORD",
    "port": "PGPORT",
    "requirepeer": "PGREQUIREPEER",
    "service": "PGSERVICE",
    "servicefile": "PGSERVICEFILE",
    "sslcert": "PGSSLCERT",
    "sslcompression": "PGSSLCOMPRESSION",
    "sslcrl": "PGSSLCRL",
    "sslcrldir": "PGSSLCRLDIR",
    "sslkey": "PGSSLKEY",
    "ssl_max_protocol_version": "PGSSLMAXPROTOCOLVERSION",
    "ssl_min_protocol_version": "PGSSLMINPROTOCOLVERSION",
    "sslmode": "PGSSLMODE",
    "sslpassword": "PGSSLPASSWORD",
    "sslrootcert": "PGSSLROOTCERT",
    "target_session_attrs": "PGTARGETSESSIONATTRS",
    "tcp_user_timeout": "PGTCPUSER_TIMEOUT",
    "user": "PGUSER",
}


def fail(message: str) -> None:
    print(message, file=sys.stderr)
    raise SystemExit(1)


def main() -> None:
    if len(sys.argv) < 2:
        fail("usage: libpq-env-exec.py <libpq-command> [arguments...]")

    database_url = os.environ.get("MIGRATION_DATABASE_URL") or os.environ.get("DATABASE_URL")
    if not database_url:
        fail("DATABASE_URL or MIGRATION_DATABASE_URL is required")

    parsed = urlsplit(database_url)
    if parsed.scheme not in {"postgres", "postgresql"}:
        fail("configured database URL must use postgres:// or postgresql://")

    env = os.environ.copy()
    for variable in LIBPQ_ENV.values():
        env.pop(variable, None)

    if parsed.hostname:
        env["PGHOST"] = unquote(parsed.hostname)
    if parsed.port:
        env["PGPORT"] = str(parsed.port)
    if parsed.username:
        env["PGUSER"] = unquote(parsed.username)
    if parsed.password is not None:
        env["PGPASSWORD"] = unquote(parsed.password)
    if parsed.path and parsed.path != "/":
        env["PGDATABASE"] = unquote(parsed.path[1:])

    for key, value in parse_qsl(parsed.query, keep_blank_values=True):
        variable = LIBPQ_ENV.get(key)
        if variable is None:
            fail(f"unsupported PostgreSQL URL parameter: {key}")
        env[variable] = value

    if "PGDATABASE" not in env:
        fail("configured database URL must name a database")

    os.execvpe(sys.argv[1], sys.argv[1:], env)


if __name__ == "__main__":
    main()
