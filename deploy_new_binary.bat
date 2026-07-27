@echo off
echo ==========================================
echo   SLAN Client V2 - Deploy New Binary
echo ==========================================
echo.

echo [1/4] Stopping SLAN Client V2 Service...
sc stop SLANClientV2Service
timeout /t 3 /nobreak >nul

echo [2/4] Stopping client process...
taskkill /f /im slan_client_v2.exe >nul 2>&1
timeout /t 2 /nobreak >nul

echo [3/5] Copying new binary...
copy /y "d:\workspace\slan\client_v2\rust\target\release\client-core-service.exe" "C:\Users\admin\AppData\Local\Programs\SLAN Client V2\client-core-service.exe"
if %errorlevel% neq 0 (
    echo ERROR: Copy failed!
    pause
    exit /b 1
)

echo [4/5] Clearing network state cache...
echo {"adapterPresent":true,"networkEnabled":false} > "%ProgramData%\SLAN\client-v2-network-state.json"

echo [5/5] Starting service...
sc start SLANClientV2Service
timeout /t 2 /nobreak >nul

echo.
echo ==========================================
echo   Deploy complete!
echo ==========================================
echo.
echo New binary: d:\workspace\slan\client_v2\rust\target\release\client-core-service.exe
echo Service binary: C:\Users\admin\AppData\Local\Programs\SLAN Client V2\client-core-service.exe
echo.
pause
