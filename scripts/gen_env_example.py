#!/usr/bin/env python3
"""Generate .env.example from .env, with every secret value blanked.

WHY GENERATED RATHER THAN MAINTAINED

`.env` holds secrets and cannot be committed, so `.env.example` is the ONLY
committed record of this project's configuration — cloning on a new machine
means reconstructing it from that file. Marking it "not maintained" was wrong
for that reason.

But hand-maintaining two copies is what produced the drift in the first place:
49 keys existed in `.env` and not `.env.example`, 55 the other way, and the
duplicate `TIINGO_RATE_PER_SEC` block meant correcting one copy silently did
nothing. Generation removes the class: there is one source, and the example is
a projection of it.

WHAT IS BLANKED

A key is treated as secret when its NAME matches a secret pattern — never by
inspecting the value, because a value that merely looks harmless today can be
a credential tomorrow, and a heuristic that guesses wrong leaks it once and
permanently. Comments and ordering are preserved so the example stays readable
as documentation.

Run:  python3 scripts/gen_env_example.py
Check: python3 scripts/gen_env_example.py --check   (non-zero if out of date)
"""
from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ENV = ROOT / ".env"
EXAMPLE = ROOT / ".env.example"

#: Substrings that make a key secret. Matched case-insensitively against the
#: KEY NAME. Deliberately broad: a false positive costs one blanked example
#: value, a false negative commits a credential to a public repository.
SECRET_MARKERS = (
    "TOKEN", "SECRET", "PASSWORD", "PASSWD", "API_KEY", "APIKEY", "KEY",
    "CREDENTIAL", "PRIVATE", "SIGNING", "WEBHOOK", "DSN", "SENTRY",
    "ACCESS_CODE", "EMERGENCY", "OAUTH", "SESSION", "SALT", "CERT",
)

#: Keys that CONTAIN a secret marker but are not secrets. Each needs a reason,
#: because the safe default is to blank.
NOT_SECRET = {
    # A rate-limit bucket name, not a credential.
    "RATELIMIT_CHILD_KEY",
    # Boolean/enable flags and tuning numbers that happen to contain "KEY".
    "TIINGO_MAX_SELECTED_SYMBOLS",
}

#: Keys whose value is not secret but IS machine-specific, so the example
#: carries a neutral placeholder rather than one developer's local setup.
LOCAL_PLACEHOLDER = {
    "DATABASE_URL": "postgres://postgres:postgres@localhost:5432/trading?sslmode=disable",
    "REDIS_URL": "redis://localhost:6379/0",
    "SEC_EDGAR_USER_AGENT": "YourProjectName admin@yourdomain.example",
}

BLANK = ""

KEY_RE = re.compile(r"^([A-Z_0-9]+)=(.*)$")


def is_secret(key: str) -> bool:
    if key in NOT_SECRET:
        return False
    up = key.upper()
    return any(m in up for m in SECRET_MARKERS)


def render(env_text: str) -> str:
    header = (
        "# ============================================================================\n"
        "# GENERATED — run: python3 scripts/gen_env_example.py\n"
        "#\n"
        "# Do NOT edit by hand. This file is a projection of .env with every secret\n"
        "# value blanked, and hand edits are overwritten on the next run.\n"
        "#\n"
        "# It is generated rather than maintained because two hand-kept copies drifted:\n"
        "# 49 keys lived in .env and not here, 55 the other way, and a duplicated\n"
        "# TIINGO_RATE_PER_SEC block meant fixing one copy silently did nothing.\n"
        "#\n"
        "# scripts/test_env_example.py fails when the key sets differ.\n"
        "# ============================================================================\n"
        "\n"
    )
    out: list[str] = []
    for line in env_text.splitlines():
        m = KEY_RE.match(line)
        if not m:
            out.append(line)          # comments and blanks pass through verbatim
            continue
        key, value = m.group(1), m.group(2)
        if key in LOCAL_PLACEHOLDER:
            out.append(f"{key}={LOCAL_PLACEHOLDER[key]}")
        elif is_secret(key):
            out.append(f"{key}={BLANK}")
        else:
            out.append(f"{key}={value}")
    return header + "\n".join(out).rstrip("\n") + "\n"


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--check", action="store_true",
                    help="exit non-zero if .env.example is out of date")
    args = ap.parse_args()

    if not ENV.exists():
        print(f"{ENV} not found", file=sys.stderr)
        return 2
    want = render(ENV.read_text(encoding="utf-8"))

    if args.check:
        have = EXAMPLE.read_text(encoding="utf-8") if EXAMPLE.exists() else ""
        if have != want:
            print(".env.example is out of date — run: python3 scripts/gen_env_example.py")
            return 1
        print(".env.example is up to date")
        return 0

    EXAMPLE.write_text(want, encoding="utf-8")
    keys = [m.group(1) for m in (KEY_RE.match(l) for l in want.splitlines()) if m]
    blanked = [k for k in keys if is_secret(k)]
    print(f"wrote {EXAMPLE.relative_to(ROOT)}: {len(keys)} keys, {len(blanked)} blanked")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
