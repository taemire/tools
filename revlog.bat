@echo off
setlocal enabledelayedexpansion

set COUNT=10
set LOG_FILE=%TEMP%\git_log_%RANDOM%.txt

:: 옵션 파싱
:parse_args
if "%~1"=="" goto :main
if /i "%~1"=="-h" goto :print_usage
if /i "%~1"=="--help" goto :print_usage
if /i "%~1"=="-n" (
    set COUNT=%~2
    shift
    shift
    goto :parse_args
)
if /i "%~1"=="--count" (
    set COUNT=%~2
    shift
    shift
    goto :parse_args
)
if /i "%~1"=="-f" set SHOW_FULL=1& shift& goto :parse_args
if /i "%~1"=="--full" set SHOW_FULL=1& shift& goto :parse_args
if /i "%~1"=="-full" set SHOW_FULL=1& shift& goto :parse_args
if /i "%~1"=="-a" set SHOW_ALL=1& shift& goto :parse_args
if /i "%~1"=="--all" set SHOW_ALL=1& shift& goto :parse_args
if /i "%~1"=="-p" set NO_PAGER=1& shift& goto :parse_args
if /i "%~1"=="--no-pager" set NO_PAGER=1& shift& goto :parse_args
shift
goto :parse_args

:print_usage
echo.
echo Usage: check_version.bat [OPTIONS]
echo.
echo Display git commit history and version information in a formatted table.
echo.
echo Options:
echo   -n, --count ^<number^>    Number of commits to display (default: 10)
echo   -a, --all               Show all commits (overrides -n)
echo   -f, --full              Show full commit details including body
echo   -p, --no-pager          Disable pagination (useful for full output)
echo   -h, --help              Display this help message
echo.
echo Examples:
echo   check_version.bat
echo   check_version.bat -n 20
echo   check_version.bat --full
echo   check_version.bat -a -f -p
echo.
goto :eof

:main
if "%SHOW_ALL%"=="1" set COUNT=999999

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

if "%SHOW_FULL%"=="1" goto :main_full

:: Use PowerShell for smart formatting
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
    "$count = %COUNT%;" ^
    "$log = @(git log --pretty=format:'%%h###SEP###%%s###SEP###%%ad' --date=format:'%%Y-%%m-%%d %%H:%%M:%%S' -n $count);" ^
    "$total = (git rev-list --count HEAD).Trim();" ^
    "$current = [int]$total;" ^
    "$fmt = '{0,-8} {1,-10} {2,-7} {3,-19} {4}';" ^
    "Write-Host ($fmt -f 'Rev', 'Tag', 'Hash', 'Time', 'Description');" ^
    "Write-Host '---      ---        ----    ----                -----------';" ^
    "foreach ($line in $log) {" ^
        "if ([string]::IsNullOrWhiteSpace($line)) { continue };" ^
        "$parts = $line -split '###SEP###';" ^
        "if ($parts.Length -ge 3) {" ^
            "$hash = $parts[0];" ^
            "$desc = $parts[1];" ^
            "$time = $parts[2];" ^
            "$tag = (git tag --points-at $hash 2>$null) -join ', ';" ^
            "if (-not $tag) { $tag = '-' };" ^
            "if ($tag.Length -gt 9) { $tag = $tag.Substring(0, 8) + '.' };" ^
            "$rev = 'r' + $current;" ^
            "Write-Host ($fmt -f $rev, $tag, $hash, $time, $desc);" ^
            "$current--;" ^
        "}" ^
    "}"

echo.
goto :footer

:main_full
set GIT_CMD=git
if "%NO_PAGER%"=="1" set GIT_CMD=git --no-pager

echo ========================================
echo   Commit History (Full Details)
echo ========================================
%GIT_CMD% log -n %COUNT% --pretty=format:"Commit: %%h (%%ad)%%nTag:    %%D%%nSummary: %%s%%n%%n%%b%%n--------------------------------------------------------------------------------" --date=short
goto :eof

:footer
:: Get total commit count for HEAD
for /f "usebackq" %%n in (`git rev-list --count HEAD`) do set TOTAL_COMMITS=%%n

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

:: 정리
if exist "%LOG_FILE%" del "%LOG_FILE%"
endlocal
