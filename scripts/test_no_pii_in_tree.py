#!/usr/bin/env python3
"""Fail if the tracked tree contains a real email address or IPv4 address.

WHY

This repository is public. Writing down the working value of a setting is the
natural thing to do when documenting a hard-won fix, and that is exactly how a
personal contact address ended up in five files and a corporate egress IP in
four: both were genuine operational findings (SEC denylists some contact
domains; the published rate limit is per-IP and ours is shared), and the honest
way to record them looked like quoting the real values.

Neither is a credential, so a secret scanner would not catch them. This checks
the other category: identifiers that are harmless internally and unwanted in
public.

    python3 scripts/test_no_pii_in_tree.py
"""
from __future__ import annotations

import re
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

EMAIL_RE = re.compile(r'\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b')
IPV4_RE = re.compile(r'\b(?:\d{1,3}\.){3}\d{1,3}\b')

#: Addresses that are allowed because they are documentation examples or
#: well-known placeholders. Anything not listed here fails, which is the right
#: default: adding an entry is a deliberate act with a reviewer, while a regex
#: broad enough to "look like an example" would let the next real one through.
EMAIL_ALLOW = {
    "you@yourdomain.example",
    "admin@yourdomain.example",
    "test@example.com",
    "user@example.com",
    "AdminContact@sample.com",
}

#: IPv4 literals that are not network identity: loopback, unspecified,
#: RFC 5737 documentation ranges, and version-like strings are handled below.
IP_ALLOW = {
    "127.0.0.1",
    "0.0.0.0",
    "255.255.255.255",
    "192.0.2.1", "198.51.100.1", "203.0.113.1",   # RFC 5737 doc ranges
}

TEXT_SUFFIXES = (".md", ".py", ".go", ".sql", ".yml", ".yaml", ".sh",
                 ".json", ".txt", ".example", ".toml", ".cfg", ".ini")

#: This file necessarily contains the patterns it searches for.
SELF = "scripts/test_no_pii_in_tree.py"


def tracked_text_files() -> list[str]:
    out = subprocess.run(["git", "ls-files"], cwd=ROOT,
                         capture_output=True, text=True).stdout.split()
    return [f for f in out if f.endswith(TEXT_SUFFIXES) and f != SELF]


def is_version_like(ip: str, line: str) -> bool:
    """Filter things that match IPv4 but are not addresses.

    Version numbers (1.2.3.4), and any octet above 255, are not addresses.
    Checked against the surrounding line so 'v1.22.3.1' is not reported.
    """
    parts = ip.split(".")
    if any(int(p) > 255 for p in parts):
        return True
    idx = line.find(ip)
    before = line[max(0, idx - 2):idx]
    return before.rstrip().endswith(("v", "V")) or "version" in line.lower()


class TestNoPIIInTree(unittest.TestCase):

    def test_no_unlisted_email_addresses(self):
        found = []
        for f in tracked_text_files():
            try:
                body = (ROOT / f).read_text(encoding="utf-8", errors="ignore")
            except Exception:
                continue
            for i, line in enumerate(body.splitlines(), 1):
                for m in EMAIL_RE.findall(line):
                    if m not in EMAIL_ALLOW:
                        found.append(f"{f}:{i}: {m}")
        self.assertEqual(
            found, [],
            "\nReal email address(es) in the tracked tree:\n  " + "\n  ".join(found[:20]) +
            "\n\nUse a placeholder such as <your-contact-email>, or add the address to "
            "EMAIL_ALLOW if it is genuinely a documentation example.")

    def test_no_unlisted_ipv4_addresses(self):
        found = []
        for f in tracked_text_files():
            try:
                body = (ROOT / f).read_text(encoding="utf-8", errors="ignore")
            except Exception:
                continue
            for i, line in enumerate(body.splitlines(), 1):
                for m in IPV4_RE.findall(line):
                    if m in IP_ALLOW or is_version_like(m, line):
                        continue
                    found.append(f"{f}:{i}: {m}")
        self.assertEqual(
            found, [],
            "\nIPv4 address(es) in the tracked tree:\n  " + "\n  ".join(found[:20]) +
            "\n\nDescribe the network instead of naming it — e.g. 'a shared corporate "
            "proxy' — or add the literal to IP_ALLOW if it is a documentation example.")

    def test_no_inline_database_passwords(self):
        """A DSN with real-looking credentials, which is neither an email nor an IP."""
        dsn = re.compile(r'(?:postgres|postgresql|mysql|redis|mongodb)://'
                         r'(?!<)[A-Za-z0-9_.-]+:(?!<)[A-Za-z0-9_.!@#$%^&*-]+@')
        found = []
        for f in tracked_text_files():
            try:
                body = (ROOT / f).read_text(encoding="utf-8", errors="ignore")
            except Exception:
                continue
            for i, line in enumerate(body.splitlines(), 1):
                for m in dsn.findall(line):
                    # postgres:postgres and unused:unused are conventional
                    # placeholders, not credentials.
                    if any(p in m for p in ("postgres:postgres@", "unused:unused@",
                                            "user:password@", "root:root@")):
                        continue
                    found.append(f"{f}:{i}: {m}")
        self.assertEqual(
            found, [],
            "\nConnection string(s) with inline credentials:\n  " + "\n  ".join(found[:20]) +
            "\n\nUse <user>:<password> placeholders.")


if __name__ == "__main__":
    unittest.main(verbosity=2)
