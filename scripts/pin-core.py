"""Pin a clean, committed core checkout for reproducible FPK builds."""
import json
import pathlib
import re
import subprocess
import sys

root = pathlib.Path(__file__).resolve().parents[1]
source = pathlib.Path(sys.argv[1]).resolve() if len(sys.argv) == 2 else None
if source is None:
    sys.exit("Usage: python3 scripts/pin-core.py /path/to/nginx-web")
def git(*args):
    return subprocess.check_output(["git", *args], cwd=source, text=True).strip()
if git("status", "--porcelain"):
    sys.exit("Commit the core changes before pinning")
version = (source / "VERSION").read_text().strip()
if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
    sys.exit("Invalid core VERSION")
if not (source / "go.mod").read_text().startswith("module github.com/chenpingonline/nginx-web\n"):
    sys.exit("Not an nginx-web core checkout")
lock = {"repository": "https://github.com/chenpingonline/nginx-web.git", "commit": git("rev-parse", "HEAD"), "version": version}
(root / "core.lock").write_text(json.dumps(lock, indent=2) + "\n")
print(f"Pinned nginx-web {version} at {lock['commit']}")
