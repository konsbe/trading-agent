#!/usr/bin/env python3
"""Fail when .env and .env.example disagree, or when a secret leaked into the example.

`.env.example` is the only committed record of this project's configuration, so
it going stale means a fresh clone silently misses settings. The generator makes
that easy to avoid; this makes it impossible to ignore.

Run: python3 scripts/test_env_example.py
"""
from __future__ import annotations

import re
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from gen_env_example import ENV, EXAMPLE, KEY_RE, LOCAL_PLACEHOLDER, is_secret, render  # noqa: E402


def keys_of(path: Path) -> dict[str, str]:
    out: dict[str, str] = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        m = KEY_RE.match(line)
        if m:
            out[m.group(1)] = m.group(2)
    return out


class TestEnvExample(unittest.TestCase):

    def test_key_sets_are_identical(self):
        """The whole point: a key in one file and not the other is drift.

        This is the check that would have caught the original 49/55 split, and
        the duplicated TIINGO_RATE_PER_SEC block that made correcting the wrong
        copy look like a change that did nothing.
        """
        env, ex = keys_of(ENV), keys_of(EXAMPLE)
        only_env = sorted(set(env) - set(ex))
        only_ex = sorted(set(ex) - set(env))
        self.assertEqual(
            (only_env, only_ex), ([], []),
            f"\n.env has {len(only_env)} key(s) missing from .env.example: {only_env[:12]}"
            f"\n.env.example has {len(only_ex)} key(s) not in .env: {only_ex[:12]}"
            f"\nRun: python3 scripts/gen_env_example.py",
        )

    def test_example_is_regenerable_byte_for_byte(self):
        """Catches hand edits, which the generator will silently overwrite."""
        self.assertEqual(
            EXAMPLE.read_text(encoding="utf-8"),
            render(ENV.read_text(encoding="utf-8")),
            "\n.env.example does not match what the generator produces — it was "
            "hand-edited or .env changed.\nRun: python3 scripts/gen_env_example.py",
        )

    def test_no_secret_value_appears_in_the_example(self):
        """The failure that actually costs something: a committed credential.

        Checked two ways — every secret-named key must be blank, AND no non-empty
        value from a secret key in .env may appear anywhere in the example text,
        which also catches a secret pasted into a comment.
        """
        env, ex = keys_of(ENV), keys_of(EXAMPLE)
        for key, value in ex.items():
            if is_secret(key):
                self.assertEqual(
                    value, "",
                    f"{key} is secret-named but carries a value in .env.example",
                )

        blob = EXAMPLE.read_text(encoding="utf-8")
        for key, value in env.items():
            if not is_secret(key) or len(value.strip()) < 8:
                continue
            self.assertNotIn(
                value.strip(), blob,
                f"the value of {key} from .env appears verbatim in .env.example",
            )

    def test_no_duplicate_keys_in_either_file(self):
        """A duplicate definition means the LAST one wins, silently.

        This repo had exactly that: TIINGO_RATE_PER_SEC defined twice in
        .env.example, so correcting the first copy changed nothing.
        """
        for path in (ENV, EXAMPLE):
            seen: dict[str, int] = {}
            for line in path.read_text(encoding="utf-8").splitlines():
                m = KEY_RE.match(line)
                if m:
                    seen[m.group(1)] = seen.get(m.group(1), 0) + 1
            dupes = {k: n for k, n in seen.items() if n > 1}
            self.assertEqual(dupes, {}, f"{path.name} defines these keys more than once: {dupes}")

    def test_machine_specific_values_are_placeholders(self):
        """A committed example must not carry one developer's local endpoints."""
        ex = keys_of(EXAMPLE)
        for key, placeholder in LOCAL_PLACEHOLDER.items():
            if key in ex:
                self.assertEqual(
                    ex[key], placeholder,
                    f"{key} in .env.example should be the neutral placeholder, not a local value",
                )


if __name__ == "__main__":
    unittest.main(verbosity=2)
