"""Read the pinned/shared core VERSION or an explicit packaged manifest."""
import pathlib,re,subprocess,sys
root=pathlib.Path(__file__).resolve().parents[1]
if len(sys.argv)>1:
    matches=re.findall(r"^version[ \t]*=[ \t]*([0-9]+\.[0-9]+\.[0-9]+)[ \t]*$",pathlib.Path(sys.argv[1]).read_text(),re.M)
    if len(matches)!=1: sys.exit("Invalid package manifest version")
    print(matches[0])
else:
    source=pathlib.Path(subprocess.check_output([sys.executable,str(root/'scripts/resolve-core.py')],text=True).strip())
    print((source/'VERSION').read_text().strip())
