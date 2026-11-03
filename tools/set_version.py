# Copyright (c) 2026 Carl Beattie

import json
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
GO_FILE = os.path.join(ROOT, "internal", "launcher", "version.go")
JSON_VERSION_FILES = [os.path.join(ROOT, "cmd", d, "versioninfo.json") for d in ("manager", "panel")]

def main():
    if len(sys.argv) != 2 or not re.fullmatch(r"\d+\.\d+\.\d+", sys.argv[1]):
        sys.exit("usage: set_version.py MAJOR.MINOR.PATCH")

    ver = sys.argv[1]

    major, minor, patch = (int(p) for p in ver.split("."))

    with open(GO_FILE, encoding="utf-8") as f:
        src = f.read()
    src, n = re.subn(r'(const Version = )"[^"]*"', r'\1"%s"' % ver, src)
    if n != 1:
        sys.exit("could not find `const Version` in " + GO_FILE)

    with open(GO_FILE, "w", encoding="utf-8", newline="") as f:
        f.write(src)

    print("wrote version change to version.go")

    for path in JSON_VERSION_FILES:
        with open(path, encoding='utf-8') as f:
            data = json.load(f)

        for key in ("FileVersion", "ProductVersion"):
            data["FixedFileInfo"][key].update(Major=major, Minor=minor, Patch=patch, Build=0)
            data["StringFileInfo"][key] = ver

        with open(path, "w", encoding="utf-8", newline="\n") as f:
            json.dump(data, f, indent=2, ensure_ascii=False)
            f.write("\n")

if __name__ == "__main__":
    main()