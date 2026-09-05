package update

import (
	"strconv"
	"strings"
)

// updateDirName is the staging folder inside the install directory.
const updateDirName = ".update"

// scriptName is the batch file that performs the swap after Burnt exits.
const scriptName = "update.cmd"

// buildUpdateScript renders the batch file that finishes an update once Burnt
// itself has exited.
//
// It cannot run while we are alive (the running burnt.exe is locked), so the
// script polls tasklist until our PID disappears, moves the staged tree over the
// install directory, relaunches the app and deletes itself. Kept portable and
// separate from Apply so it can be unit-tested off Windows.
func buildUpdateScript(pid int, updateDir, installDir, exe string) string {
	var b strings.Builder
	w := func(lines ...string) {
		for _, l := range lines {
			b.WriteString(l)
			b.WriteString("\r\n") // cmd.exe is happiest with CRLF
		}
	}

	w(
		"@echo off",
		"setlocal",
		"set PID="+strconv.Itoa(pid),
		`set "UPDATE_DIR=`+updateDir+`"`,
		`set "INSTALL_DIR=`+installDir+`"`,
		`set "EXE=`+exe+`"`,
		"",
		"rem Wait for the running Burnt to exit so its files are unlocked.",
		"set TRIES=0",
		":waitloop",
		`tasklist /FI "PID eq %PID%" | find "%PID%" >nul 2>&1`,
		"if errorlevel 1 goto swap",
		"set /a TRIES+=1",
		"if %TRIES% GEQ 120 goto swap",
		"ping -n 2 127.0.0.1 >nul 2>&1",
		"goto waitloop",
		"",
		":swap",
		`robocopy "%UPDATE_DIR%" "%INSTALL_DIR%" /E /IS /IT /MOVE /NFL /NDL /NJH /NJS /NP >nul 2>&1`,
		"if errorlevel 8 (",
		`  xcopy "%UPDATE_DIR%\*" "%INSTALL_DIR%\" /E /Y /I >nul 2>&1`,
		")",
		`if exist "%UPDATE_DIR%" rmdir /S /Q "%UPDATE_DIR%" >nul 2>&1`,
		"",
		"rem Relaunch and remove this script (the goto trick lets cmd delete the",
		"rem batch file it is currently executing).",
		`start "" "%EXE%"`,
		`(goto) 2>nul & del "%~f0"`,
	)
	return b.String()
}
