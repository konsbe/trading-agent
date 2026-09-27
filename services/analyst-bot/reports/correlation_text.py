"""Fundamental-correlation sentences as shown, from shared/content.

fundamental-analysis stores each aligned / divergent sentence with an
implication its comparison does not test ("cascade to insolvency risk",
"compounding machine"). shared/content/correlation_sentences.json maps each
stored format string to a description of the comparison that was met;
momentum-api serves the same file to the web app. A number in the stored
sentence (%.1f) is carried into the shown text as rendered. A sentence the file
does not know is shown as stored.
"""
from __future__ import annotations

import re
from typing import Iterable, List

from reports import shared_content

CorrelationTextError = shared_content.SharedContentError


def _load() -> list[tuple[re.Pattern, str]]:
    data = shared_content.load("CORRELATION_TEXT_PATH", "correlation_sentences.json")
    sentences = data.get("sentences") if isinstance(data, dict) else None
    if not sentences:
        raise CorrelationTextError("correlation_sentences.json has no sentences")
    rules = []
    for s in sentences:
        stored, display = s.get("stored"), s.get("display")
        if not stored or not display:
            raise CorrelationTextError("correlation_sentences.json has an entry with an empty stored or display text")
        pattern = re.escape(stored).replace(r"%\.1f", r"(-?[0-9]+(?:\.[0-9]+)?)").replace("%%", "%")
        rules.append((re.compile("^" + pattern + "$"), display))
    return rules


_RULES = _load()


def display(stored: str) -> str:
    """The shown text for one stored sentence."""
    for pattern, text in _RULES:
        m = pattern.match(stored)
        if m:
            return text % m.groups()
    return stored


def displays(stored: Iterable[str]) -> List[str]:
    return [display(s) for s in stored if isinstance(s, str)]
