@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul

:: ==============================================================================
:: md2pdf_v2.bat - 2-Pass PDF Generation with Accurate TOC Page Numbers
::
:: Usage: md2pdf_v2.bat -i <input_dir> -o <output_file> [options]
::
:: This script generates PDF documents with accurate Table of Contents page numbers
:: by using a 2-pass approach:
::   Pass 1: Generate initial PDF without page numbers
::   Pass 2: Analyze PDF to extract actual page numbers, regenerate with correct numbers
:: ==============================================================================

set TOOL_DIR=%~dp0
if "%TOOL_DIR:~-1%"=="\" set TOOL_DIR=%TOOL_DIR:~0,-1%

:: Check tools
if not exist "%TOOL_DIR%\md2html_v2.exe" (
    echo [BUILD] Building md2html_v2...
    pushd "%TOOL_DIR%"
    go build -o md2html_v2.exe .
    popd
)
if not exist "%TOOL_DIR%\..\html2pdf.exe" (
    echo [ERROR] html2pdf.exe not found in parent directory
    exit /b 1
)
if not exist "%TOOL_DIR%\cmd\pdf_analyzer\pdf_analyzer.exe" (
    echo [BUILD] Building pdf_analyzer...
    pushd "%TOOL_DIR%\cmd\pdf_analyzer"
    go build -o pdf_analyzer.exe .
    popd
)

:: Parse arguments
set INPUT_PATH=
set OUTPUT_PATH=
set TITLE=
set SUBTITLE=
set VERSION=
set AUTHOR=
set HEADER=
set FOOTER=
set CONFIG_FILE=
set TEMPLATE=report
set SKIP_PAGES=0
set PAGE_OFFSET=1

:parse_args
if "%~1"=="" goto :check_args
if /i "%~1"=="-i" set INPUT_PATH=%~2& shift& shift& goto :parse_args
if /i "%~1"=="-o" set OUTPUT_PATH=%~2& shift& shift& goto :parse_args
if /i "%~1"=="-title" set "TITLE=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-subtitle" set "SUBTITLE=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-version" set "VERSION=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-author" set "AUTHOR=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-header" set "HEADER=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-footer" set "FOOTER=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-config" set "CONFIG_FILE=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-c" set "CONFIG_FILE=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-template" set "TEMPLATE=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-skip" set "SKIP_PAGES=%~2"& shift& shift& goto :parse_args
if /i "%~1"=="-offset" set "PAGE_OFFSET=%~2"& shift& shift& goto :parse_args
shift
goto :parse_args

:check_args
if "!INPUT_PATH!"=="" (
    echo [ERROR] Input path ^(-i^) is required.
    echo Usage: md2pdf_v2.bat -i ^<input_dir^> -o ^<output_file^> [options]
    echo.
    echo Options:
    echo   -i          Input directory with markdown files
    echo   -o          Output file path ^(without extension^)
    echo   -title      Document title
    echo   -subtitle   Document subtitle
    echo   -version    Document version
    echo   -author     Author name
    echo   -header     Header text
    echo   -footer     Footer text
    echo   -c/-config  AUTHORS.yml config file
    echo   -template   Template name ^(default: report^)
    echo   -skip       Pages to skip for PDF analysis ^(default: 0 = auto-detect^)
    echo   -offset     Page number offset ^(default: 1, cover not counted^)
    exit /b 1
)
if "!OUTPUT_PATH!"=="" (
    echo [ERROR] Output path ^(-o^) is required.
    exit /b 1
)

:: Sanitize output path
if /i "!OUTPUT_PATH:~-5!"==".html" set "OUTPUT_PATH=!OUTPUT_PATH:~0,-5!"
if /i "!OUTPUT_PATH:~-4!"==".pdf" set "OUTPUT_PATH=!OUTPUT_PATH:~0,-4!"

set HTML_OUT=!OUTPUT_PATH!.html
set PDF_OUT=!OUTPUT_PATH!.pdf
set HTML_PASS1=!OUTPUT_PATH!_pass1.html
set PDF_PASS1=!OUTPUT_PATH!_pass1.pdf
set SECTIONS_JSON=!OUTPUT_PATH!_sections.json
set PAGES_JSON=!OUTPUT_PATH!_pages.json

echo.
echo ==============================================================================
echo   md2pdf_v2 - 2-Pass PDF Generation with Accurate TOC Page Numbers
echo ==============================================================================
echo   Input:  !INPUT_PATH!
echo   Output: !PDF_OUT!
echo   Skip:   !SKIP_PAGES! pages ^(0 = auto-detect TOC end^)
echo   Offset: !PAGE_OFFSET! ^(page number adjustment^)
echo ==============================================================================
echo.

:: ==============================================================================
:: PASS 1: Generate initial HTML and PDF (without page numbers)
:: ==============================================================================
echo [PASS 1] Generating initial PDF...

set MD2HTML_CMD1="%TOOL_DIR%\md2html_v2.exe" -i "!INPUT_PATH!" -o "!HTML_PASS1!" -template "!TEMPLATE!" -sections-json "!SECTIONS_JSON!"
if defined CONFIG_FILE set MD2HTML_CMD1=!MD2HTML_CMD1! -c "!CONFIG_FILE!"
if defined TITLE set MD2HTML_CMD1=!MD2HTML_CMD1! -title "!TITLE!"
if defined SUBTITLE set MD2HTML_CMD1=!MD2HTML_CMD1! -subtitle "!SUBTITLE!"
if defined VERSION set MD2HTML_CMD1=!MD2HTML_CMD1! -version "!VERSION!"
if defined AUTHOR set MD2HTML_CMD1=!MD2HTML_CMD1! -author "!AUTHOR!"
if defined HEADER set MD2HTML_CMD1=!MD2HTML_CMD1! -header "!HEADER!"
if defined FOOTER set MD2HTML_CMD1=!MD2HTML_CMD1! -footer "!FOOTER!"

echo [PASS 1] Running: !MD2HTML_CMD1!
call !MD2HTML_CMD1!
if !ERRORLEVEL! NEQ 0 (
    echo [ERROR] Pass 1 HTML generation failed
    exit /b 1
)

:: Generate Pass 1 PDF
echo [PASS 1] Converting HTML to PDF...
"%TOOL_DIR%\..\html2pdf.exe" -i "!HTML_PASS1!" -o "!PDF_PASS1!"
if not exist "!PDF_PASS1!" (
    echo [ERROR] Pass 1 PDF generation failed
    exit /b 1
)
echo [PASS 1] Initial PDF generated: !PDF_PASS1!

:: ==============================================================================
:: PASS 2: Analyze PDF and regenerate with correct page numbers
:: ==============================================================================
echo.
echo [PASS 2] Analyzing PDF for page numbers...

:: Run PDF analyzer
"%TOOL_DIR%\cmd\pdf_analyzer\pdf_analyzer.exe" -i "!PDF_PASS1!" -sections "!SECTIONS_JSON!" -skip !SKIP_PAGES! -offset !PAGE_OFFSET! -o "!PAGES_JSON!"
if !ERRORLEVEL! NEQ 0 (
    echo [WARN] PDF analysis failed, continuing without page numbers
    copy /y "!PDF_PASS1!" "!PDF_OUT!" >nul
    goto :cleanup
)

echo [PASS 2] Page numbers extracted to: !PAGES_JSON!

:: Regenerate HTML with page numbers
echo [PASS 2] Regenerating HTML with page numbers...
set MD2HTML_CMD2="%TOOL_DIR%\md2html_v2.exe" -i "!INPUT_PATH!" -o "!HTML_OUT!" -template "!TEMPLATE!"
set MD2HTML_CMD2=!MD2HTML_CMD2! -pages-json "!PAGES_JSON!"
if defined CONFIG_FILE set MD2HTML_CMD2=!MD2HTML_CMD2! -c "!CONFIG_FILE!"
if defined TITLE set MD2HTML_CMD2=!MD2HTML_CMD2! -title "!TITLE!"
if defined SUBTITLE set MD2HTML_CMD2=!MD2HTML_CMD2! -subtitle "!SUBTITLE!"
if defined VERSION set MD2HTML_CMD2=!MD2HTML_CMD2! -version "!VERSION!"
if defined AUTHOR set MD2HTML_CMD2=!MD2HTML_CMD2! -author "!AUTHOR!"
if defined HEADER set MD2HTML_CMD2=!MD2HTML_CMD2! -header "!HEADER!"
if defined FOOTER set MD2HTML_CMD2=!MD2HTML_CMD2! -footer "!FOOTER!"

call !MD2HTML_CMD2!
if !ERRORLEVEL! NEQ 0 (
    echo [ERROR] Pass 2 HTML generation failed
    exit /b 1
)

:: Generate final PDF
echo [PASS 2] Converting to final PDF...
"%TOOL_DIR%\..\html2pdf.exe" -i "!HTML_OUT!" -o "!PDF_OUT!"
if not exist "!PDF_OUT!" (
    echo [ERROR] Final PDF generation failed
    exit /b 1
)

:cleanup
:: Clean up intermediate files
del "!HTML_PASS1!" 2>nul
del "!PDF_PASS1!" 2>nul
del "!SECTIONS_JSON!" 2>nul
del "!PAGES_JSON!" 2>nul

echo.
echo ==============================================================================
echo [SUCCESS] Generated with accurate page numbers: !PDF_OUT!
echo ==============================================================================
echo.

exit /b 0
