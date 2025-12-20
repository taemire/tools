@echo off
setlocal enabledelayedexpansion

set COUNT=10
set LOG_FILE=%TEMP%\git_log_%RANDOM%.txt

:: 옵션 파싱
:parse_args
if "%~1"=="" goto :main
if /i "%~1"=="-n" (
    set COUNT=%~2
    shift
    shift
    goto :parse_args
    goto :parse_args
)
if /i "%~1"=="-full" (
    goto :main_full
)
shift
goto :parse_args

:main_full
echo.
echo ========================================
echo   Commit History (Full Details)
echo ========================================
echo.
git log -n %COUNT% --pretty=format:"Commit: %%h (%%ad)%%nTag:    %%D%%nSummary: %%s%%n%%n%%b%%n--------------------------------------------------------------------------------" --date=short
echo.
goto :eof

:main
:: 1. 리포지토리 정보 가져오기
for /f "usebackq delims=" %%i in (`git rev-parse --show-toplevel 2^>nul`) do set "REPO_PATH=%%i"
if not defined REPO_PATH (
    echo [ERROR] Git repository not found.
    goto :eof
)
:: 경로에서 리포지토리명 추출 (마지막 폴더명)
for %%i in ("%REPO_PATH%") do set "REPO_NAME=%%~nxi"

echo.
echo [%REPO_NAME%] %CD%
echo.

:: Use PowerShell for smart formatting
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
    "$count = %COUNT%;" ^
    "$log = git log --pretty=format:'%%h|%%s|%%ad' --date=format:'%%Y-%%m-%%d %%H:%%M:%%S' -n $count;" ^
    "$total = (git rev-list --count HEAD).Trim();" ^
    "$current = [int]$total;" ^
    "$log | ForEach-Object {" ^
        "$parts = $_ -split '\|';" ^
        "$hash = $parts[0];" ^
        "$desc = $parts[1];" ^
        "$time = $parts[2];" ^
        "$tag = (git tag --points-at $hash 2>$null) -join ', ';" ^
        "if (-not $tag) { $tag = '-' };" ^
        "[PSCustomObject]@{" ^
            "Rev = 'r' + $current;" ^
            "Tag = $tag;" ^
            "Hash = $hash;" ^
            "Time = $time;" ^
            "Description = $desc" ^
        "};" ^
        "$current--;" ^
    "} | Format-Table -Property @{N='Rev';E={$_.Rev};Width=8}, @{N='Tag';E={$_.Tag};Width=16}, @{N='Hash';E={$_.Hash};Width=12}, @{N='Time';E={$_.Time};Width=21}, @{N='Description';E={$_.Description}} -AutoSize"

echo.
for /f %%i in ('git rev-parse --short HEAD') do set HEAD_HASH=%%i
:: HEAD에 태그가 있는지 확인
set "HEAD_TAG="
for /f "usebackq delims=" %%t in (`git tag --points-at HEAD 2^>nul`) do (
    if not defined HEAD_TAG set "HEAD_TAG=%%t"
)
if defined HEAD_TAG (
    echo Current HEAD: r%TOTAL_COMMITS% [!HEAD_TAG!] ^(%HEAD_HASH%^)
) else (
    echo Current HEAD: r%TOTAL_COMMITS% ^(%HEAD_HASH%^)
)
echo.

:: 정리
if exist "%LOG_FILE%" del "%LOG_FILE%"
endlocal
