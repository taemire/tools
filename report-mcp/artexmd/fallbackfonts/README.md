# Bundled CJK fallback fonts

The renderer embeds these fonts only when a base face lacks a visible character.
They are static 400/700 WOFF2 instances of Noto Sans JP and Noto Sans SC, licensed
under the SIL Open Font License. The complete licenses are included beside the
fonts and in the generated HTML font-license metadata.

Upstream: https://github.com/google/fonts/tree/6157dfaa640ca5738b4d1a7070fe389c3036d7fc/ofl

`manifest.json` records the pinned upstream URLs and hashes, the derived asset
hashes, PostScript names and Unicode coverage. The Go loader pins the complete
manifest hash as well as checking each embedded font. The report service needs
neither Python nor font downloads at runtime.

To reproduce the assets in a new directory:

```sh
python -m venv /absolute/font-build-venv
/absolute/font-build-venv/bin/pip install fonttools==4.68.0 brotli==1.2.1
/absolute/font-build-venv/bin/python scripts/build-fallback-fonts.py \
  --source-dir /absolute/source-cache --download --output /absolute/new-output
```

The script checks every upstream source and byte-compares the complete rebuilt
manifest, including coverage and identities. A mismatch fails the build. Font
updates require source/license review, regeneration and a new reviewed manifest
pin in `artexmd/font_fallback.go`.
