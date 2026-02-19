#!/usr/bin/env bash
# ==============================================================================
# md2pdf_v2.sh - Wrapper for md2pdf unified binary
#
# Usage: md2pdf_v2.sh -i <input_dir> -o <output_file> [options]
#
# This script is a thin wrapper around the unified md2pdf binary.
# It auto-builds md2pdf if not found and forwards all arguments.
#
# NOTE: This script is maintained for backward compatibility.
#       Direct usage of md2pdf binary is recommended.
# ==============================================================================

set -eu

# --- Tool directories ---
TOOL_DIR="$(cd "$(dirname "$0")" && pwd)"
MD2PDF_DIR="${TOOL_DIR}/md2pdf"
MD2PDF_BIN="${MD2PDF_DIR}/md2pdf"

# --- Color codes ---
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

# --- Build md2pdf if not found ---
if [ ! -f "$MD2PDF_BIN" ]; then
    echo -e "${BLUE}[BUILD] Building md2pdf...${NC}"
    if [ ! -d "$MD2PDF_DIR" ]; then
        echo -e "${RED}[ERROR] md2pdf source directory not found: ${MD2PDF_DIR}${NC}"
        exit 1
    fi
    pushd "$MD2PDF_DIR" > /dev/null
    go build -o md2pdf .
    popd > /dev/null
    if [ ! -f "$MD2PDF_BIN" ]; then
        echo -e "${RED}[ERROR] Failed to build md2pdf${NC}"
        exit 1
    fi
    echo -e "${GREEN}[BUILD] md2pdf built successfully${NC}"
fi

# --- Forward all arguments to md2pdf ---
exec "$MD2PDF_BIN" "$@"
