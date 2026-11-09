@echo off
rem ---------------------------------------------------------------------------
rem namefix build script
rem
rem Builds two executables from the same sources with the C# compiler that ships
rem with .NET Framework 4.x (no SDK required):
rem   bin\namefix.exe   console subsystem  - terminal, scripts, tests
rem   bin\namefixw.exe  windows subsystem  - Explorer context menu (no console window)
rem
rem This file is intentionally ASCII-only. cmd.exe reads batch files with the OEM
rem code page, so UTF-8 Korean text in a .bat file can corrupt the following line.
rem Korean documentation lives in README.md and in the C# sources.
rem ---------------------------------------------------------------------------
setlocal
set "ROOT=%~dp0"
set "CSC=%WINDIR%\Microsoft.NET\Framework64\v4.0.30319\csc.exe"
if not exist "%CSC%" set "CSC=%WINDIR%\Microsoft.NET\Framework\v4.0.30319\csc.exe"
if not exist "%CSC%" (
  echo [namefix] csc.exe not found. .NET Framework 4.x is required.
  exit /b 1
)
if not exist "%ROOT%src\Program.cs" (
  echo [namefix] source not found: %ROOT%src
  exit /b 1
)
if not exist "%ROOT%bin" mkdir "%ROOT%bin"

set "OPTS=/nologo /codepage:65001 /optimize+ /warnaserror+ /platform:anycpu /r:System.dll /r:System.Core.dll /r:System.Windows.Forms.dll"

"%CSC%" %OPTS% /target:exe /out:"%ROOT%bin\namefix.exe" "%ROOT%src\*.cs"
if errorlevel 1 exit /b 1

"%CSC%" %OPTS% /target:winexe /define:WINEXE /out:"%ROOT%bin\namefixw.exe" "%ROOT%src\*.cs"
if errorlevel 1 exit /b 1

echo [namefix] build ok: %ROOT%bin
endlocal
exit /b 0
