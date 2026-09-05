"""Read the application version from the source or a packaged fnOS manifest."""
import pathlib
import re
import sys

path = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else pathlib.Path(__file__).resolve().parents[1] / "packaging/fnos/manifest"
matches = re.findall(r"^version[ \t]*=[ \t]*([0-9]+\.[0-9]+\.[0-9]+)[ \t]*$", path.read_text(), re.MULTILINE)
if len(matches) != 1:
    sys.exit("manifest must contain exactly one version in major.minor.patch format")
print(matches[0])
