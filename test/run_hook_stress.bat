@REM Hook-system stability scenarios (h* plugin family). Each scenario is a
@REM standalone plugin that builds its own private pools and asserts PASS/FAIL
@REM invariants; any FAIL makes the scenario exit 1 and the script aborts.
@REM Run with the race detector:  run_hook_stress.bat race
@REM   (race builds a -race binary; slower, use it in CI to catch hook races)
@REM Per-plugin knobs: see `agilepool_test.exe --help hcount` etc.
if "%1"=="race" (
    go build -race -o agilepool_test.exe .
) else (
    go build -o agilepool_test.exe .
)
if errorlevel 1 exit /b 1

.\agilepool_test.exe --hcount
if errorlevel 1 exit /b 1
.\agilepool_test.exe --hpanic num=3000 stage=all
if errorlevel 1 exit /b 1
.\agilepool_test.exe --hpanic num=200 level=callback stage=all 2>nul
if errorlevel 1 exit /b 1
.\agilepool_test.exe --horder
if errorlevel 1 exit /b 1
.\agilepool_test.exe --hctx
if errorlevel 1 exit /b 1
.\agilepool_test.exe --hblock num=2000 delay-us=100
if errorlevel 1 exit /b 1
.\agilepool_test.exe --hchurn
if errorlevel 1 exit /b 1
.\agilepool_test.exe --hreenter
if errorlevel 1 exit /b 1
.\agilepool_test.exe --hclose
if errorlevel 1 exit /b 1

@echo hook-stability suite: all PASS
