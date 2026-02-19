#!/bin/bash

COUNT=10
SHOW_FULL=0
SHOW_ALL=0
NO_PAGER=0

# 옵션 파싱
while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)
            echo
            echo "Usage: revlog.sh [OPTIONS]"
            echo
            echo "Display git commit history and version information in a formatted table."
            echo
            echo "Options:"
            echo "  -n, --count <number>    Number of commits to display (default: 10)"
            echo "  -a, --all               Show all commits (overrides -n)"
            echo "  -f, --full, -full       Show full commit details including body"
            echo "  -p, --no-pager          Disable pagination (useful for full output)"
            echo "  -h, --help              Display this help message"
            echo
            echo "Examples:"
            echo "  revlog.sh"
            echo "  revlog.sh -n 20"
            echo "  revlog.sh --full"
            echo "  revlog.sh -a -f -p"
            echo
            exit 0
            ;;
        -n|--count)
            COUNT="$2"
            shift 2
            ;;
        -f|--full|-full)
            SHOW_FULL=1
            shift
            ;;
        -a|--all)
            SHOW_ALL=1
            shift
            ;;
        -p|--no-pager)
            NO_PAGER=1
            shift
            ;;
        *)
            shift
            ;;
    esac
done

# --all 이면 커밋 수 제한 해제
if [[ "$SHOW_ALL" -eq 1 ]]; then
    COUNT=999999
fi

# 리포지토리 정보 가져오기
REPO_PATH=$(git rev-parse --show-toplevel 2>/dev/null)
if [[ -z "$REPO_PATH" ]]; then
    echo "[ERROR] Git repository not found."
    exit 1
fi
REPO_NAME=$(basename "$REPO_PATH")

echo
echo "[$REPO_NAME] $(pwd)"
echo

# Full 모드
if [[ "$SHOW_FULL" -eq 1 ]]; then
    GIT_CMD="git"
    if [[ "$NO_PAGER" -eq 1 ]]; then
        GIT_CMD="git --no-pager"
    fi

    echo "========================================"
    echo "  Commit History (Full Details)"
    echo "========================================"
    $GIT_CMD log -n "$COUNT" --pretty=format:"Commit: %h (%ad)%nTag:    %D%nSummary: %s%n%n%b%n--------------------------------------------------------------------------------" --date=short
    exit 0
fi

# 일반 테이블 모드
TOTAL=$(git rev-list --count HEAD)
CURRENT=$TOTAL
SEP='###SEP###'

# git log 결과 수집 및 동적 컬럼 너비 계산
MAX_TAG_LEN=3
MAX_REV_LEN=${#TOTAL}
((MAX_REV_LEN++)) # 'r' 접두사 포함
if [[ $MAX_REV_LEN -lt 3 ]]; then
    MAX_REV_LEN=3
fi

ENTRIES=()
while IFS= read -r line; do
    [[ -z "$line" ]] && continue

    hash="${line%%${SEP}*}"; rest="${line#*${SEP}}"
    desc="${rest%%${SEP}*}"; rest="${rest#*${SEP}}"
    time="${rest%%${SEP}*}"; refs="${rest#*${SEP}}"

    tag="-"
    if [[ -n "$refs" ]]; then
        # 태그 추출
        extracted=$(echo "$refs" | grep -oE 'tag: [^,)]+' | sed 's/tag: //' | paste -sd ', ' -)
        if [[ -n "$extracted" ]]; then
            tag="$extracted"
        fi
    fi

    if [[ ${#tag} -gt $MAX_TAG_LEN ]]; then
        MAX_TAG_LEN=${#tag}
    fi

    ENTRIES+=("r${CURRENT}${SEP}${tag}${SEP}${hash}${SEP}${time}${SEP}${desc}")
    ((CURRENT--))
done < <(git log --pretty=format:"%h${SEP}%s${SEP}%ad${SEP}%D" --date=format:'%Y-%m-%d %H:%M:%S' -n "$COUNT")

# 헤더 출력
printf "%-${MAX_REV_LEN}s %-${MAX_TAG_LEN}s %-7s %-19s %s\n" "Rev" "Tag" "Hash" "Time" "Description"
printf "%s %s %s %s %s\n" \
    "$(printf '%*s' "$MAX_REV_LEN" '' | tr ' ' '-')" \
    "$(printf '%*s' "$MAX_TAG_LEN" '' | tr ' ' '-')" \
    "$(printf '%*s' 7 '' | tr ' ' '-')" \
    "$(printf '%*s' 19 '' | tr ' ' '-')" \
    "$(printf '%*s' 30 '' | tr ' ' '-')"

# 데이터 출력
for entry in "${ENTRIES[@]}"; do
    rev="${entry%%${SEP}*}"; rest="${entry#*${SEP}}"
    tag="${rest%%${SEP}*}"; rest="${rest#*${SEP}}"
    hash="${rest%%${SEP}*}"; rest="${rest#*${SEP}}"
    time="${rest%%${SEP}*}"; desc="${rest#*${SEP}}"

    printf "%-${MAX_REV_LEN}s %-${MAX_TAG_LEN}s %-7s %-19s %s\n" "$rev" "$tag" "$hash" "$time" "$desc"
done

echo

# Footer
TOTAL_COMMITS=$(git rev-list --count HEAD)
HEAD_HASH=$(git rev-parse --short HEAD)
HEAD_TAG=$(git tag --points-at HEAD 2>/dev/null | head -n1)

if [[ -n "$HEAD_TAG" ]]; then
    echo "Current HEAD: r${TOTAL_COMMITS} [${HEAD_TAG}] (${HEAD_HASH})"
else
    echo "Current HEAD: r${TOTAL_COMMITS} (${HEAD_HASH})"
fi
