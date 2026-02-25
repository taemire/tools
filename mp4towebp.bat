@echo off
SETLOCAL EnableDelayedExpansion

REM ============================================
REM  MP4 to WebP Converter (using FFmpeg)
REM ============================================
REM  Usage: mp4towebp.bat input.mp4 [output.webp] [fps] [width]
REM  Example: mp4towebp.bat video.mp4 output.webp 15 480
REM ============================================

SET "INPUT=%~1"
SET "OUTPUT=%~2"
SET "FPS=%~3"
SET "WIDTH=%~4"

IF "!INPUT!"=="" (
    echo Usage: mp4towebp.bat input.mp4 [output.webp] [fps] [width]
    echo.
    echo   input.mp4    - Source video file
    echo   output.webp  - Output WebP file (default: input_name.webp)
    echo   fps          - Frame rate (default: 15)
    echo   width        - Output width in pixels (default: 480, height auto)
    echo.
    echo Example: mp4towebp.bat demo.mp4 demo.webp 10 320
    EXIT /B 1
)

REM Check if FFmpeg is installed
SET "FFMPEG_CMD=ffmpeg"
where ffmpeg >nul 2>&1
IF ERRORLEVEL 1 (
    IF EXIST "%~dp0ffmpeg.exe" (
        SET "FFMPEG_CMD=%~dp0ffmpeg.exe"
    ) ELSE (
        echo [ERROR] FFmpeg not found in system PATH or current directory.
        EXIT /B 1
    )
)

IF "!OUTPUT!"=="" (
    FOR %%i IN ("!INPUT!") DO SET "OUTPUT=%%~ni.webp"
)
IF "!FPS!"=="" SET "FPS=15"
IF "!WIDTH!"=="" SET "WIDTH=480"

echo ============================================
echo  MP4 to WebP Converter
echo  Input:  !INPUT!
echo  Output: !OUTPUT!
echo  FPS:    !FPS!
echo  Width:  !WIDTH!px
echo ============================================

"!FFMPEG_CMD!" -y -i "!INPUT!" -vf "fps=!FPS!,scale=!WIDTH!:-1:flags=lanczos" -vcodec libwebp -lossless 0 -compression_level 6 -q:v 70 -loop 0 -preset default -an -vsync 0 "!OUTPUT!"

IF ERRORLEVEL 1 (
    echo.
    echo Conversion failed!
    EXIT /B 1
)

echo.
echo ✓ Conversion complete: !OUTPUT!

FOR %%A IN ("!OUTPUT!") DO (
    SET "SIZE=%%~zA"
    SET /A "SIZEKB=!SIZE!/1024"
    echo   File size: !SIZEKB! KB
)

EXIT /B 0
