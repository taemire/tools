#!/usr/bin/env python3
"""Rebuild the bundled OFL fonts from hash-pinned upstream sources.

Build-only dependencies: fonttools==4.68.0, brotli==1.2.1.
No font download or Python dependency is used by the report service at runtime.
"""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import urllib.request

import brotli
import fontTools
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-dir", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True, help="new output directory")
    parser.add_argument("--download", action="store_true", help="download pinned sources if missing")
    args = parser.parse_args()
    base = Path(__file__).resolve().parent.parent / "artexmd/fallbackfonts"
    original = json.loads((base / "manifest.json").read_text())
    if fontTools.__version__ != "4.68.0" or brotli.__version__ != "1.2.1":
        raise SystemExit("Use fonttools==4.68.0 and brotli==1.2.1 for reproducible assets")
    if args.output.exists():
        raise SystemExit("Output must be a new directory")
    args.source_dir.mkdir(parents=True, exist_ok=True)
    for source in original["source"]["files"]:
        target = args.source_dir / source["file"]
        if target.name != source["file"]:
            raise SystemExit("Invalid source name")
        if not target.exists() and args.download:
            with urllib.request.urlopen(source["url"], timeout=60) as response:
                data = response.read(24 << 20)
            if hashlib.sha256(data).hexdigest() != source["sha256"]:
                raise SystemExit("Downloaded source hash mismatch")
            target.write_bytes(data)
        if hashlib.sha256(target.read_bytes()).hexdigest() != source["sha256"]:
            raise SystemExit("Source hash mismatch")
    args.output.mkdir(parents=True)
    manifest = {"source": original["source"], "fonttools_version": fontTools.__version__, "families": []}
    for code in ["JP", "SC"]:
        source = args.source_dir / f"NotoSans{code}-NotoSans{code}[wght].ttf"
        family = {"name": "ReportCJK" + code, "language": code.lower(), "fonts": []}
        for weight in [400, 700]:
            face = TTFont(source, recalcTimestamp=False)
            face = instantiateVariableFont(face, {"wght": weight}, inplace=True, updateFontNames=True)
            face.recalcTimestamp = False
            face["head"].modified = face["head"].created
            face.flavor = "woff2"
            name = f"NotoSans{code}-{weight}.woff2"
            output = args.output / name
            face.save(output)
            readback = TTFont(output)
            points = sorted(cp for cp, glyph in readback.getBestCmap().items() if glyph != ".notdef")
            ranges = []
            for point in points:
                if ranges and ranges[-1][1] + 1 == point:
                    ranges[-1][1] = point
                else:
                    ranges.append([point, point])
            family["fonts"].append({"weight": weight, "file": name,
                "sha256": hashlib.sha256(output.read_bytes()).hexdigest(),
                "bytes": output.stat().st_size, "ranges": ranges, "postscript_name": readback["name"].getDebugName(6)})
        shutil.copyfile(args.source_dir / f"NotoSans{code}-OFL.txt", args.output / f"NotoSans{code}-OFL.txt")
        manifest["families"].append(family)
    (args.output / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, separators=(",", ":")) + "\n")
    for family in manifest["families"]:
        for face in family["fonts"]:
            expected = next(f for a in original["families"] for f in a["fonts"] if f["file"] == face["file"])
            if face["sha256"] != expected["sha256"]:
                raise SystemExit("Derived font differs from the pinned asset: " + face["file"])
    if (args.output / "manifest.json").read_bytes() != (base / "manifest.json").read_bytes():
        raise SystemExit("Coverage or font metadata differs from the pinned manifest")
    print("Bundled fonts and complete coverage manifest reproduced byte-for-byte")


if __name__ == "__main__":
    main()
