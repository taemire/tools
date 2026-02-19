#!/bin/bash
# ============================================
#  Video to WebP Converter (macOS/Linux)
# ============================================
#  Usage: video2webp.sh <input> [output] [fps] [width]
#  Example: video2webp.sh demo.mov demo.webp 1 1280
# ============================================
#  Dependencies: ffmpeg, img2webp (brew install ffmpeg webp)
# ============================================

set -euo pipefail

INPUT="${1:-}"
OUTPUT="${2:-}"
FPS="${3:-1}"
WIDTH="${4:-1280}"
QUALITY="${5:-75}"

if [[ -z "$INPUT" ]]; then
    cat <<'USAGE'
Usage: video2webp.sh <input> [output] [fps] [width] [quality]

  input    - Source video file (mov, mp4, avi, mkv, webm)
  output   - Output WebP file (default: <input_name>.webp)
  fps      - Frame rate (default: 1, 분석용)
  width    - Output width in pixels (default: 1280 = 720p)
  quality  - WebP quality 0-100 (default: 75)

Examples:
  video2webp.sh screen.mov                       # 기본: 1fps, 720p
  video2webp.sh demo.mp4 out.webp 2 1920 85      # 2fps, 1080p, 고품질
  video2webp.sh record.mov analysis.webp 1 1280   # 분석용 1fps, 720p

Frame delay = 1000/fps ms (1fps → 1000ms, 2fps → 500ms)
USAGE
    exit 1
fi

# --- Validate input ---
if [[ ! -f "$INPUT" ]]; then
    echo "[ERROR] File not found: $INPUT"
    exit 1
fi

# --- Check dependencies ---
for cmd in ffmpeg img2webp; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "[ERROR] $cmd not found. Install: brew install ffmpeg webp"
        exit 1
    fi
done

# --- Defaults ---
if [[ -z "$OUTPUT" ]]; then
    OUTPUT="${INPUT%.*}.webp"
fi

DELAY=$(( 1000 / FPS ))
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# --- Video info ---
DURATION=$(ffprobe -v quiet -print_format json -show_format "$INPUT" \
    | python3 -c "import json,sys; print(f\"{float(json.load(sys.stdin)['format']['duration']):.1f}\")")
ORIGINAL_SIZE=$(du -h "$INPUT" | cut -f1)

echo "============================================"
echo " Video → WebP Converter"
echo " Input:   $INPUT ($ORIGINAL_SIZE, ${DURATION}s)"
echo " Output:  $OUTPUT"
echo " FPS:     $FPS (delay: ${DELAY}ms)"
echo " Width:   ${WIDTH}px"
echo " Quality: $QUALITY"
echo "============================================"

# --- Step 1: Extract frames ---
echo "[1/3] Extracting frames (${FPS}fps, ${WIDTH}px)..."
ffmpeg -y -i "$INPUT" \
    -vf "fps=$FPS,scale=$WIDTH:-1:flags=lanczos" \
    "$TMPDIR/frame_%04d.png" 2>&1 | tail -1

FRAME_COUNT=$(ls "$TMPDIR"/frame_*.png 2>/dev/null | wc -l | tr -d ' ')
echo "      → $FRAME_COUNT frames extracted"

if [[ "$FRAME_COUNT" -eq 0 ]]; then
    echo "[ERROR] No frames extracted"
    exit 1
fi

# --- Step 2: Assemble animated WebP ---
echo "[2/3] Assembling animated WebP..."
img2webp -lossy -q "$QUALITY" -d "$DELAY" "$TMPDIR"/frame_*.png -o "$OUTPUT" 2>&1

# --- Step 3: Report ---
WEBP_SIZE=$(du -h "$OUTPUT" | cut -f1)
WEBP_BYTES=$(wc -c < "$OUTPUT" | tr -d ' ')
ORIG_BYTES=$(wc -c < "$INPUT" | tr -d ' ')
RATIO=$(python3 -c "print(f'{(1 - $WEBP_BYTES/$ORIG_BYTES)*100:.1f}')")

echo "[3/3] Done!"
echo ""
echo "============================================"
echo " Result"
echo "   Original: $ORIGINAL_SIZE ($INPUT)"
echo "   WebP:     $WEBP_SIZE ($OUTPUT)"
echo "   Ratio:    ${RATIO}% reduced"
echo "   Frames:   $FRAME_COUNT"
echo "============================================"
