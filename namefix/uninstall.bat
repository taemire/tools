@echo off
rem ---------------------------------------------------------------------------
rem namefix uninstaller
rem
rem   1. Remove the Explorer context menu entries (namefix.exe --unregister)
rem   2. Delete %LOCALAPPDATA%\Programs\namefix
rem
rem Registry removal always goes through namefix.exe --unregister, which treats a
rem missing key as success, reports access-denied as failure, and verifies that
rem no menu key is left. If the installed namefix.exe is gone, the one in bin\
rem next to this script is used (built first if necessary).
rem
rem Exit code 0 only when both steps succeed. The rename log
rem (%LOCALAPPDATA%\namefix\namefix.log) is kept so that past renames can
rem still be reviewed or reverted.
rem This file is intentionally ASCII-only and uses no labels (see build.bat).
rem ---------------------------------------------------------------------------
setlocal
set "ROOT=%~dp0"
set "DEST=%LOCALAPPDATA%\Programs\namefix"
set "TOOL=%DEST%\namefix.exe"
if not exist "%TOOL%" set "TOOL=%ROOT%bin\namefix.exe"
if not exist "%TOOL%" call "%ROOT%build.bat"
if not exist "%TOOL%" (
  echo [namefix] namefix.exe not found and the build failed. Nothing was removed.
  exit /b 1
)

"%TOOL%" --unregister
if errorlevel 1 (
  echo [namefix] failed to remove the Explorer menu entries. Files were left in place.
  exit /b 1
)

if exist "%DEST%" rmdir /s /q "%DEST%"
if exist "%DEST%" (
  echo [namefix] could not delete %DEST%
  echo [namefix] close any namefix window that is still open and run uninstall.bat again.
  exit /b 1
)

echo [namefix] uninstalled. Log kept: %LOCALAPPDATA%\namefix\namefix.log
endlocal
exit /b 0
