@echo off
rem ---------------------------------------------------------------------------
rem namefix installer (current user only, no administrator rights required)
rem
rem   1. Build bin\namefix.exe and bin\namefixw.exe (build.bat)
rem   2. Copy them to %LOCALAPPDATA%\Programs\namefix
rem   3. Register the Explorer context menu under HKCU\Software\Classes
rem
rem Run again after updating the sources to reinstall. Remove with uninstall.bat.
rem This file is intentionally ASCII-only (see build.bat).
rem ---------------------------------------------------------------------------
setlocal
set "ROOT=%~dp0"
set "DEST=%LOCALAPPDATA%\Programs\namefix"

call "%ROOT%build.bat"
if errorlevel 1 exit /b 1

if not exist "%DEST%" mkdir "%DEST%"
if errorlevel 1 exit /b 1

copy /y "%ROOT%bin\namefix.exe" "%DEST%\namefix.exe" >nul
if errorlevel 1 (
  echo [namefix] copy failed. Close any running namefix window and try again.
  exit /b 1
)
copy /y "%ROOT%bin\namefixw.exe" "%DEST%\namefixw.exe" >nul
if errorlevel 1 (
  echo [namefix] copy failed. Close any running namefix window and try again.
  exit /b 1
)

"%DEST%\namefix.exe" --register
if errorlevel 1 exit /b 1

echo [namefix] installed: %DEST%
endlocal
exit /b 0
