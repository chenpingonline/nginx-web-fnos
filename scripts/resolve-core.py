"""Resolve a reproducible core checkout; an explicit path enables local development."""
import fcntl
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[1]


def git(*args, cwd=None):
    return subprocess.check_output(["git", *args], cwd=cwd, text=True, stderr=subprocess.PIPE).strip()


def validate(source, expected=None):
    source = pathlib.Path(source).resolve()
    version = (source / "VERSION").read_text().strip()
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        raise ValueError("Invalid core VERSION")
    if not (source / "go.mod").read_text().startswith("module github.com/chenpingonline/nginx-web\n"):
        raise ValueError("Not an nginx-web core checkout")
    if expected:
        if git("rev-parse", "HEAD", cwd=source) != expected["commit"] or version != expected["version"]:
            raise ValueError("Core checkout does not match core.lock")
        if git("status", "--porcelain", cwd=source):
            raise ValueError("Pinned core checkout was modified; use NGINX_WEB_CORE for development")
    return source


def resolve():
    override = os.getenv("NGINX_WEB_CORE")
    if override:
        return validate(override)
    lock = json.loads((ROOT / "core.lock").read_text())
    if not re.fullmatch(r"[0-9a-f]{40}", lock["commit"]):
        raise ValueError("core.lock needs a full commit SHA")
    if not lock["repository"].startswith("https://github.com/") or not lock["repository"].endswith(".git"):
        raise ValueError("Invalid core repository URL")
    cache = ROOT / ".cache/core"
    cache.mkdir(parents=True, exist_ok=True)
    with (cache / ".resolve.lock").open("a") as handle:
        fcntl.flock(handle, fcntl.LOCK_EX)
        target = cache / lock["commit"]
        if target.exists():
            return validate(target, lock)
        origin = lock["repository"]
        for candidate in (ROOT.parent / "nginx-web", ROOT.parent.parent / "nginx-web"):
            try:
                validate(candidate)
                git("cat-file", "-e", lock["commit"] + "^{commit}", cwd=candidate)
                origin = str(candidate)
                break
            except (OSError, ValueError, subprocess.CalledProcessError):
                pass
        temp = pathlib.Path(tempfile.mkdtemp(prefix="checkout-", dir=cache))
        try:
            git("clone", "--quiet", "--no-local", "--no-checkout", "--single-branch", origin, str(temp))
            try:
                git("cat-file", "-e", lock["commit"] + "^{commit}", cwd=temp)
            except subprocess.CalledProcessError:
                git("fetch", "--quiet", "--depth=1", "origin", lock["commit"], cwd=temp)
            git("checkout", "--quiet", "--detach", lock["commit"], cwd=temp)
            git("remote", "set-url", "origin", lock["repository"], cwd=temp)
            validate(temp, lock)
            temp.rename(target)
        finally:
            if temp.exists():
                shutil.rmtree(temp)
        return target


if __name__ == "__main__":
    try:
        print(resolve())
    except (OSError, ValueError, KeyError, subprocess.CalledProcessError) as error:
        detail = error.stderr if isinstance(error, subprocess.CalledProcessError) else str(error)
        sys.exit("Unable to resolve nginx-web core: " + detail.strip())
