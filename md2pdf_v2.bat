@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul

:: ==============================================================================
:: md2pdf_v2.bat - Wrapper for md2pdf unified binary
::
:: Usage: md2pdf_v2.bat -i <input_dir> -o <output_file> [options]
::
:: This script is a thin wrapper around the unified md2pdf binary.
:: It auto-builds md2pdf if not found and forwards all arguments.
::
:: NOTE: This script is maintained for backward compatibility.
::       Direct usage of md2pdf binary is recommended.
:: ==============================================================================

set TOOL_DIR=%~dp0
if "%TOOL_DIR:~-1%"=="\" set TOOL_DIR=%TOOL_DIR:~0,-1%

set MD2PDF_DIR=%TOOL_DIR%\md2pdf
set MD2PDF_BIN=%MD2PDF_DIR%\md2pdf.exe

:: Build md2pdf if not found
if not exist "%MD2PDF_BIN%" (
    echo [BUILD] Building md2pdf...
    if not exist "%MD2PDF_DIR%" (
        echo [ERROR] md2pdf source directory not found: %MD2PDF_DIR%
        exit /b 1
    )
    pushd "%MD2PDF_DIR%"
    go build -o md2pdf.exe .
    popd
    if not exist "%MD2PDF_BIN%" (
        echo [ERROR] Failed to build md2pdf
        exit /b 1
    )
    echo [BUILD] md2pdf built successfully
)

:: Forward all arguments to md2pdf
"%MD2PDF_BIN%" %*

exit /b %ERRORLEVEL%
