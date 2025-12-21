@echo off
setlocal enabledelayedexpansion

:: ==============================================================================
:: Markdown to PDF Converter Wrapper (md2html + html2pdf)
::
:: Usage:
::   md2pdf.bat -i <input_path> -o <output_file_path> [options]
::
:: Options:
::   -i <path>       : Input directory or file path (Markdown)
::   -o <path>       : Output file path (without extension, or with .pdf/.html)
::   -title <text>   : Document title
::   -subtitle <text>: Document subtitle
::   -version <text> : Version string
::   -author <text>  : Author name
::   -template <name>: Template name (default: report)
::
:: Example:
::   md2pdf.bat -i docs\manual -o dist\User_Manual -title "My App" -version 1.0
:: ==============================================================================

:: 1. Check & Build Basic Tools
set TOOL_DIR=%~dp0
if "%TOOL_DIR:~-1%"=="\" set TOOL_DIR=%TOOL_DIR:~0,-1%

if not exist "%TOOL_DIR%\md2html.exe" call :build_md2html
if not exist "%TOOL_DIR%\html2pdf.exe" call :build_html2pdf

:: 2. Parse Arguments
set INPUT_PATH=
set OUTPUT_PATH=
set CONFIG_FILE=
set TITLE=
set SUBTITLE=
set DOC_VERSION=
set AUTHOR=
set TEMPLATE=report

:parse_args
if "%~1"=="" goto :check_args
if /i "%~1"=="-i" set INPUT_PATH=%~2& shift& shift& goto :parse_args
if /i "%~1"=="-o" set OUTPUT_PATH=%~2& shift& shift& goto :parse_args
if /i "%~1"=="-c" set "CONFIG_FILE=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="--config" set "CONFIG_FILE=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-title" set "TITLE=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-subtitle" set "SUBTITLE=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-version" set "DOC_VERSION=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-author" set "AUTHOR=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-template" set "TEMPLATE=%~2"& shift& shift& goto :parse_args
shift
goto :parse_args

:build_md2html
echo [TOOLS] md2html.exe not found. Building...
pushd "%TOOL_DIR%\md2html"
go build -o ..\md2html.exe .
popd
if not exist "%TOOL_DIR%\md2html.exe" (
    echo [ERROR] Failed to build md2html.exe
    exit /b 1
)
echo [TOOLS] md2html.exe built successfully.
exit /b 0

:build_html2pdf
echo [TOOLS] html2pdf.exe not found. Building...
pushd "%TOOL_DIR%\html2pdf"
go build -o ..\html2pdf.exe .
popd
if not exist "%TOOL_DIR%\html2pdf.exe" (
    echo [ERROR] Failed to build html2pdf.exe
    exit /b 1
)
echo [TOOLS] html2pdf.exe built successfully.
exit /b 0

:check_args
if "!INPUT_PATH!"=="" (
    echo [ERROR] Input path ^(-i^) is required.
    exit /b 1
)
if "!OUTPUT_PATH!"=="" (
    echo [ERROR] Output path ^(-o^) is required.
    exit /b 1
)

:: Sanitize Output Path (Remove extension if present to enforce consistency)
if /i "!OUTPUT_PATH:~-5!"==".html" set "OUTPUT_PATH=!OUTPUT_PATH:~0,-5!"
if /i "!OUTPUT_PATH:~-4!"==".pdf" set "OUTPUT_PATH=!OUTPUT_PATH:~0,-4!"

set HTML_OUT=!OUTPUT_PATH!.html
set PDF_OUT=!OUTPUT_PATH!.pdf

:: Create output directory if it doesn't exist
for %%I in ("!OUTPUT_PATH!") do set OUTPUT_DIR=%%~dpI
if not exist "!OUTPUT_DIR!" mkdir "!OUTPUT_DIR!"

:: 3. Run md2html
echo [DOCS] Converting Markdown to HTML...
echo   - Input: !INPUT_PATH!
echo   - Output: !HTML_OUT!

set MD2HTML_CMD="%TOOL_DIR%\md2html.exe" -i "!INPUT_PATH!" -o "!HTML_OUT!" -template "!TEMPLATE!"
if defined CONFIG_FILE set MD2HTML_CMD=!MD2HTML_CMD! -c "!CONFIG_FILE!"
if defined TITLE set MD2HTML_CMD=!MD2HTML_CMD! -title "!TITLE!"
if defined SUBTITLE set MD2HTML_CMD=!MD2HTML_CMD! -subtitle "!SUBTITLE!"
if defined DOC_VERSION set MD2HTML_CMD=!MD2HTML_CMD! -version "!DOC_VERSION!"
if defined AUTHOR set MD2HTML_CMD=!MD2HTML_CMD! -author "!AUTHOR!"

call !MD2HTML_CMD!
if !ERRORLEVEL! NEQ 0 (
    echo [ERROR] md2html conversion failed.
    exit /b 1
)

:: 4. Run html2pdf
echo [DOCS] Converting HTML to PDF...
echo   - Input: !HTML_OUT!
echo   - Output: !PDF_OUT!

"%TOOL_DIR%\html2pdf.exe" -i "!HTML_OUT!" -o "!PDF_OUT!" 2>nul
if exist "!PDF_OUT!" (
    echo [SUCCESS] Generated: !PDF_OUT!
) else (
    echo [WARN] PDF generation failed or skipped ^(Chrome required^).
)

exit /b 0
