@echo off
SETLOCAL EnableDelayedExpansion

REM ============================================
REM  MP4 to WebP Converter (using FFmpeg)
REM ============================================
REM  Usage: mp4towebp.bat input.mp4 [output.webp] [fps] [width]
REM  Example: mp4towebp.bat video.mp4 output.webp 15 480
REM ============================================

SET INPUT=%1
SET OUTPUT=%2
SET FPS=%3
SET WIDTH=%4

IF "%INPUT%"=="" (
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

REM Check if FFmpeg is installed (Priority: Local > System)
SET FFMPEG_CMD=ffmpeg
IF EXIST "%~dp0ffmpeg.exe" (
    SET FFMPEG_CMD="%~dp0ffmpeg.exe"
) ELSE (
    where ffmpeg >nul 2>&1
    IF ERRORLEVEL 1 (
        echo [WARN] FFmpeg not found in system PATH or current directory.
        set /p INSTALL_CONFIRM="Do you want to download and install FFmpeg locally? (Y/N): "
        IF /I "!INSTALL_CONFIRM!"=="Y" (
            echo.
            echo [DOWN] Downloading FFmpeg (approx. 25MB)...
            echo        Source: https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip
            
            powershell -Command "Invoke-WebRequest -Uri 'https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip' -OutFile '%~dp0ffmpeg.zip'"
            IF ERRORLEVEL 1 (
                echo [ERROR] Download failed. Please install manually.
                EXIT /B 1
            )
            
            echo [EXTR] Extracting ffmpeg.exe...
            powershell -Command "Expand-Archive -Path '%~dp0ffmpeg.zip' -DestinationPath '%~dp0temp_ffmpeg' -Force"
            
            REM Find ffmpeg.exe in extracted folder and move it
            FOR /R "%~dp0temp_ffmpeg" %%F IN (ffmpeg.exe) DO (
                MOVE /Y "%%F" "%~dp0ffmpeg.exe" >nul
            )
            
            REM Cleanup
            DEL "%~dp0ffmpeg.zip"
            RMDIR /S /Q "%~dp0temp_ffmpeg"
            
            IF EXIST "%~dp0ffmpeg.exe" (
                SET FFMPEG_CMD="%~dp0ffmpeg.exe"
                echo [SUCC] FFmpeg installed successfully!
                echo.
            ) ELSE (
                echo [ERROR] Failed to locate ffmpeg.exe after extraction.
                EXIT /B 1
            )
        ) ELSE (
            echo [INFO] Operation cancelled by user.
            EXIT /B 1
        )
    )
)

REM Set defaults
IF "%OUTPUT%"=="" (
    FOR %%i IN ("%INPUT%") DO SET OUTPUT=%%~ni.webp
)
IF "%FPS%"=="" SET FPS=15
IF "%WIDTH%"=="" SET WIDTH=480

echo ============================================
echo  MP4 to WebP Converter
echo ============================================
echo  Input:  %INPUT%
echo  Output: %OUTPUT%
echo  FPS:    %FPS%
echo  Width:  %WIDTH%px
echo ============================================

REM Convert using FFmpeg
echo.
echo Converting...
echo Converting...
%FFMPEG_CMD% -y -i "%INPUT%" -vf "fps=%FPS%,scale=%WIDTH%:-1:flags=lanczos" -vcodec libwebp -lossless 0 -compression_level 6 -q:v 70 -loop 0 -preset default -an -vsync 0 "%OUTPUT%"

IF ERRORLEVEL 1 (
    echo.
    echo Conversion failed!
    EXIT /B 1
)

echo.
echo ✓ Conversion complete: %OUTPUT%

REM Show file size
FOR %%A IN ("%OUTPUT%") DO (
    SET SIZE=%%~zA
    SET /A SIZEKB=!SIZE!/1024
    echo   File size: !SIZEKB! KB
)

EXIT /B 0
