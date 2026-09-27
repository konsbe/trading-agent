"""Loader for text files in shared/content that momentum-api also serves.

Each such file is the single source of some wording both surfaces show, so
there is deliberately no fallback copy: a missing or malformed file fails at
import, loudly, like the momentum caveats.
"""
from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any


class SharedContentError(RuntimeError):
    """A shared/content file is missing or malformed. Raised at import."""


def load(env_var: str, filename: str) -> Any:
    """Read shared/content/<filename>, or the path in env_var when it is set."""
    fix = (
        f"It is shared with momentum-api so the Discord report and the web app "
        f"use identical wording. Fix: in Docker, mount the repo's shared/content "
        f"read-only and set {env_var}=/shared/content/{filename} (see "
        f"infra/docker-compose.yml, service analyst-bot). Running from a repo "
        f"checkout, leave it unset."
    )
    configured = os.environ.get(env_var)
    if configured:
        path = Path(configured)
    else:
        # services/analyst-bot/reports/<module>.py -> repo root; a Docker image
        # (/app/reports/) has no repo root above it, so it must set the variable.
        parents = Path(__file__).resolve().parents
        if len(parents) <= 3:
            raise SharedContentError(f"{env_var} is not set and this is not a repo checkout. {fix}")
        path = parents[3] / "shared" / "content" / filename
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise SharedContentError(f"Shared content file not found at {path} ({env_var}). {fix}") from exc
    except (OSError, ValueError) as exc:
        raise SharedContentError(f"Shared content file at {path} could not be read: {exc}. {fix}") from exc
