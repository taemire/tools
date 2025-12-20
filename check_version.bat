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

:: 2. 전체 커밋 개수 구하기
for /f %%i in ('git rev-list --count HEAD 2^>nul') do set TOTAL_COMMITS=%%i
if not defined TOTAL_COMMITS (
    echo [ERROR] Failed to count commits.
    goto :eof
)

:: 3. Git 로그 파일 생성 (최신 순) --reverse 제거
git log --pretty=format:"%%h|%%s" -n %COUNT% > "%LOG_FILE%"

echo.
echo [!REPO_NAME!] !REPO_PATH!
echo.
echo Rev     Tag        Hash       Description
echo --------------------------------------------------------------------------------

:: 현재 인덱스는 최신 커밋(TOTAL)부터 시작
set /a CURRENT_REV=TOTAL_COMMITS

for /f "usebackq tokens=1* delims=|" %%a in ("%LOG_FILE%") do (
    set "HASH=%%a"
    set "DESC=%%b"
    
    :: 설명 길이 제한 (약 55자)
    if "!DESC:~55,1!" neq "" set "DESC=!DESC:~0,52!..."
    
    :: 해당 커밋에 태그가 있는지 확인
    set "TAG=-"
    for /f "usebackq delims=" %%t in (`git tag --points-at !HASH! 2^>nul`) do (
        set "TAG=%%t"
    )
    
    :: Rev 번호 형식 (r9, r8...)
    set "REV=r!CURRENT_REV!      "
    set "REV=!REV:~0,7!"
    
    :: Tag 형식 (최대 10자)
    set "TAG_FMT=!TAG!          "
    set "TAG_FMT=!TAG_FMT:~0,10!"
    
    set "HASH_FMT=!HASH!          "
    set "HASH_FMT=!HASH_FMT:~0,10!"
    
    echo !REV! !TAG_FMT! !HASH_FMT! !DESC!
    
    :: 인덱스 감소
    set /a CURRENT_REV-=1
)

echo --------------------------------------------------------------------------------
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
