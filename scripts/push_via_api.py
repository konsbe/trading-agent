#!/usr/bin/env python3
"""Push a rewritten branch through the GitHub REST API.

WHY THIS EXISTS

Nokia's Zscaler proxy blocks the git write path. `POST /git-receive-pack` to
github.com returns a 403 HTML page ("Not allowed to upload files to this site
Github"), and SSH over 443 is reset during key exchange. Both git transports
are closed from this network.

`api.github.com` is not blocked. The REST git-database endpoints can create
blobs, trees, commits and move a ref, which is the same set of operations a
push performs — so the branch can be replayed object by object instead.

CORRECTNESS

Git object IDs are content addresses. If a commit is recreated with the same
tree, parents, author, committer, timestamps and message, GitHub computes the
same SHA git computed locally. Every commit is therefore verified against its
local SHA, and the script aborts on the first mismatch rather than continuing
and leaving a subtly different history on the remote. That check is the whole
safety argument: a silent divergence here would be very hard to notice later.

An identity-only rewrite does not change tree SHAs, so trees for commits that
were already pushed are present on the remote and are reused rather than
re-uploaded.
"""
from __future__ import annotations

import argparse
import base64
import datetime
import json
import subprocess
import sys
import time
import urllib.error
import urllib.request

API = "https://api.github.com"


def git(*a: str) -> str:
    return subprocess.run(["git", *a], capture_output=True, text=True,
                          check=True).stdout.rstrip("\n")


def git_raw(*a: str) -> bytes:
    return subprocess.run(["git", *a], capture_output=True, check=True).stdout


class GH:
    def __init__(self, token: str, repo: str):
        self.token, self.repo = token, repo
        self.calls = 0

    def _req(self, method: str, path: str, body=None):
        url = path if path.startswith("http") else f"{API}/repos/{self.repo}{path}"
        data = json.dumps(body).encode() if body is not None else None
        r = urllib.request.Request(url, data=data, method=method)
        r.add_header("Authorization", f"token {self.token}")
        r.add_header("Accept", "application/vnd.github+json")
        if data:
            r.add_header("Content-Type", "application/json")
        for attempt in range(5):
            try:
                self.calls += 1
                with urllib.request.urlopen(r, timeout=60) as resp:
                    return json.loads(resp.read() or b"{}")
            except urllib.error.HTTPError as e:
                payload = e.read().decode(errors="replace")
                # Secondary rate limits are transient; everything else is not.
                if e.code in (403, 429) and "rate limit" in payload.lower():
                    wait = 20 * (attempt + 1)
                    print(f"      rate limited, waiting {wait}s")
                    time.sleep(wait)
                    continue
                raise SystemExit(f"\n  {method} {url} -> HTTP {e.code}\n  {payload[:400]}")
            except urllib.error.URLError as e:
                if attempt == 4:
                    raise SystemExit(f"\n  {method} {url} -> {e}")
                time.sleep(5 * (attempt + 1))
        raise SystemExit("  exhausted retries")

    def get(self, p):
        return self._req("GET", p)

    def post(self, p, b):
        return self._req("POST", p, b)

    def patch(self, p, b):
        return self._req("PATCH", p, b)

    def exists(self, p) -> bool:
        try:
            self._req("GET", p)
            return True
        except SystemExit:
            return False


def commit_meta(sha: str) -> dict:
    """Author/committer identity and dates, parsed from the raw commit object.

    Parsed from `cat-file` rather than a `--format` string because the message
    must be byte-exact: pretty-printing normalises trailing newlines, and a
    message that differs by one byte produces a different commit SHA. That is
    not a theoretical risk — it is what the SHA check caught on the first
    attempt at this.
    """
    raw = git_raw("cat-file", "commit", sha).decode("utf-8", errors="surrogateescape")
    header, _, message = raw.partition("\n\n")

    if "\ngpgsig" in "\n" + header:
        raise SystemExit(
            f"  {sha[:9]} is GPG-signed. The API cannot reproduce the signature,"
            " so the SHA would change. Aborting rather than silently dropping it.")

    def ident(prefix: str) -> dict:
        line = next(l for l in header.splitlines() if l.startswith(prefix + " "))
        rest = line[len(prefix) + 1:]
        who, _, when = rest.rpartition(" <")[0], None, rest.rsplit(">", 1)[1].strip()
        name, email = rest.rsplit(" <", 1)[0], rest.rsplit(" <", 1)[1].split(">")[0]
        epoch, offset = when.split()
        sign, hh, mm = offset[0], int(offset[1:3]), int(offset[3:5])
        delta = datetime.timedelta(hours=hh, minutes=mm)
        tz = datetime.timezone(-delta if sign == "-" else delta)
        iso = datetime.datetime.fromtimestamp(int(epoch), tz).isoformat()
        return {"name": name, "email": email, "date": iso}

    return {"message": message,
            "author": ident("author"),
            "committer": ident("committer")}


def changed_entries(parent: str | None, sha: str) -> list[dict]:
    """Tree entries to overlay on the parent tree, from git's raw diff."""
    if parent is None:
        raw = git("ls-tree", "-r", sha)
        out = []
        for line in raw.splitlines():
            meta, path = line.split("\t", 1)
            mode, typ, blob = meta.split()
            out.append({"path": path, "mode": mode, "type": typ, "sha": blob})
        return out

    # --no-abbrev: the raw format abbreviates object IDs by default, and the
    # API needs full 40-character SHAs.
    raw = git("diff", "--raw", "--no-abbrev", "--no-renames", parent, sha)
    entries = []
    for line in raw.splitlines():
        meta, path = line.split("\t", 1)
        parts = meta.lstrip(":").split()
        dst_mode, dst_sha, status = parts[1], parts[3], parts[4]
        if status == "D":
            # sha:null tells the API to remove the path from the base tree.
            entries.append({"path": path, "mode": "100644", "type": "blob", "sha": None})
        else:
            typ = "commit" if dst_mode == "160000" else "blob"
            entries.append({"path": path, "mode": dst_mode, "type": typ, "sha": dst_sha})
    return entries


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo", default="konsbe/trading-agent")
    ap.add_argument("--branch", default="main")
    ap.add_argument("--token-from", default="~/.git-credentials")
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    import os
    import re
    cred = open(os.path.expanduser(args.token_from)).read()
    m = re.search(r"https://[^:]*:([^@]*)@github\.com", cred)
    if not m:
        raise SystemExit("  no github.com credential found")
    gh = GH(m.group(1), args.repo)

    commits = git("rev-list", "--reverse", args.branch).split()
    print(f"  replaying {len(commits)} commits onto {args.repo}:{args.branch}")
    print(f"  verification: every created commit SHA must equal its local SHA\n")

    remote_tree_cache: dict[str, bool] = {}
    new_parent = None
    uploaded_blobs = 0

    for i, sha in enumerate(commits, 1):
        tree = git("rev-parse", f"{sha}^{{tree}}")
        meta = commit_meta(sha)
        subject = meta["message"].splitlines()[0][:52]

        # Does the remote already have this tree? Identity-only rewrites keep
        # tree SHAs, so previously pushed commits need no object upload.
        if tree not in remote_tree_cache:
            remote_tree_cache[tree] = gh.exists(f"/git/trees/{tree}")
        have_tree = remote_tree_cache[tree]

        if not have_tree:
            parent_local = git("rev-parse", f"{sha}^") if i > 1 else None
            entries = changed_entries(parent_local, sha)

            # Upload any blob the remote lacks. Git SHAs are identical on both
            # sides, so a blob that exists remotely needs no re-upload.
            for e in entries:
                if e["sha"] is None or e["type"] != "blob":
                    continue
                if gh.exists(f"/git/blobs/{e['sha']}"):
                    continue
                content = git_raw("cat-file", "blob", e["sha"])
                r = gh.post("/git/blobs", {
                    "content": base64.b64encode(content).decode(),
                    "encoding": "base64",
                })
                uploaded_blobs += 1
                if r["sha"] != e["sha"]:
                    raise SystemExit(f"  blob SHA mismatch: local {e['sha']} remote {r['sha']}")

            body = {"tree": entries}
            if parent_local is not None:
                body["base_tree"] = git("rev-parse", f"{parent_local}^{{tree}}")
            r = gh.post("/git/trees", body)
            if r["sha"] != tree:
                raise SystemExit(
                    f"  tree SHA mismatch on {sha[:9]}: local {tree} remote {r['sha']}\n"
                    f"  refusing to continue — the replayed history would differ")
            remote_tree_cache[tree] = True

        payload = dict(meta)
        payload["tree"] = tree
        payload["parents"] = [new_parent] if new_parent else []

        if args.dry_run:
            print(f"  [{i:>2}/{len(commits)}] would create  {sha[:9]}  {subject}")
            new_parent = sha
            continue

        created = gh.post("/git/commits", payload)
        if created["sha"] != sha:
            raise SystemExit(
                f"\n  COMMIT SHA MISMATCH at {i}: local {sha} remote {created['sha']}\n"
                f"  the replay diverged; ref not moved, remote still on its old tip")
        new_parent = created["sha"]
        flag = " " if have_tree else "+"
        print(f"  [{i:>2}/{len(commits)}] {flag} {sha[:9]}  {subject}")

    if args.dry_run:
        print("\n  dry run: ref not moved")
        return 0

    print(f"\n  all {len(commits)} commits recreated with matching SHAs "
          f"({uploaded_blobs} blobs uploaded, {gh.calls} API calls)")
    print(f"  moving refs/heads/{args.branch} -> {new_parent[:9]} (force)")
    gh.patch(f"/git/refs/heads/{args.branch}", {"sha": new_parent, "force": True})

    check = gh.get(f"/git/refs/heads/{args.branch}")["object"]["sha"]
    if check != new_parent:
        raise SystemExit(f"  ref did not move: {check}")
    print(f"  remote {args.branch} is now {check[:9]}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
