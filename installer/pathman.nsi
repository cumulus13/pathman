; ─────────────────────────────────────────────────────────────────────────────
; PathMan Windows installer (NSIS 3, Unicode)
;
; Build:
;   makensis -DVERSION=1.0.5 -DVERSION_NUM=1.0.5.0 -DARCH=amd64 ^
;            -DEXE=dist\pathman.exe -DSRCDIR=. -DOUTFILE=pathman_setup.exe ^
;            installer\pathman.nsi
;   ARCH = amd64 | 386 | arm64
;
; Silent:    setup.exe /S [/D=C:\path\without\quotes]      (/D must be last)
;            uninstall.exe /S
;
; PATH is NOT edited by NSIS. NSIS strings are limited to 1024 chars (a long
; PATH would be truncated) and NSIS writes REG_SZ (which would flatten a
; REG_EXPAND_SZ PATH). The installer calls "pathman.exe path add|remove"
; instead, which keeps the value type and takes an automatic backup first.
; ─────────────────────────────────────────────────────────────────────────────
Unicode true

!ifndef VERSION
  !error "pass -DVERSION=x.y.z"
!endif
!ifndef ARCH
  !error "pass -DARCH=amd64|386|arm64"
!endif
!ifndef EXE
  !error "pass -DEXE=path\to\pathman.exe"
!endif
!ifndef OUTFILE
  !define OUTFILE "pathman_${VERSION}_windows_${ARCH}_setup.exe"
!endif
!ifndef SRCDIR
  !define SRCDIR ".."
!endif

!define APPNAME   "PathMan"
!define PUBLISHER "cumulus13"
!define URL       "https://github.com/cumulus13/pathman"
!define APP_KEY   "Software\PathMan"
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\PathMan"

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "WinMessages.nsh"

Name "${APPNAME} ${VERSION}"
OutFile "${OUTFILE}"
BrandingText "${APPNAME} ${VERSION} (${ARCH})"
RequestExecutionLevel admin
SetCompressor /SOLID lzma
ShowInstDetails show
ShowUninstDetails show

!if "${ARCH}" == "386"
  InstallDir "$PROGRAMFILES32\${APPNAME}"
!else
  InstallDir "$PROGRAMFILES64\${APPNAME}"
!endif

!ifdef VERSION_NUM
  VIProductVersion "${VERSION_NUM}"
  VIAddVersionKey "ProductName" "${APPNAME}"
  VIAddVersionKey "CompanyName" "${PUBLISHER}"
  VIAddVersionKey "FileDescription" "${APPNAME} installer"
  VIAddVersionKey "FileVersion" "${VERSION}"
  VIAddVersionKey "ProductVersion" "${VERSION}"
  VIAddVersionKey "LegalCopyright" "${PUBLISHER}"
!endif

Var PrevDir

; ── pages ────────────────────────────────────────────────────────────────────
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_TITLE "PathMan is installed"
!define MUI_FINISHPAGE_TEXT "Open a NEW terminal window so the updated PATH takes effect, then run:$\r$\n$\r$\n    pathman --help"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "${SRCDIR}\LICENSE"
!insertmacro MUI_PAGE_COMPONENTS
!define MUI_PAGE_CUSTOMFUNCTION_LEAVE DirLeave
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

; ── helpers ──────────────────────────────────────────────────────────────────
Function .onInit
  !if "${ARCH}" != "386"
    SetRegView 64
  !endif

  ; Upgrade: default to the folder of the existing install, unless /D= was given.
  ; (InstallDirRegKey is not used: it is read before SetRegView takes effect.)
  ReadRegStr $PrevDir HKLM "${APP_KEY}" "InstallDir"
  !if "${ARCH}" == "386"
    StrCpy $R0 "$PROGRAMFILES32\${APPNAME}"
  !else
    StrCpy $R0 "$PROGRAMFILES64\${APPNAME}"
  !endif
  ${If} $PrevDir != ""
  ${AndIf} $INSTDIR == $R0
    StrCpy $INSTDIR $PrevDir
  ${EndIf}

  ; The directory page is skipped in silent mode, so validate here as well.
  ${If} ${Silent}
    Push $INSTDIR
    Call ValidateDir
    Pop $0
    ${If} $0 != 0
      SetErrorLevel 2
      Abort
    ${EndIf}
  ${EndIf}
FunctionEnd

; Characters that would corrupt PATH (;) or make "path remove" match as a glob
; (* ? [ ]) or break command-line quoting (") are not allowed in the folder.
Function ValidateDir
  Exch $0           ; $0 = directory, old $0 is on the stack
  Push $1
  Push $2
  Push $3
  Push $4
  StrCpy $3 0       ; 0 = ok, 1 = bad
  StrLen $2 $0
  ${If} $2 < 4      ; drive roots such as C: (3 chars)
    StrCpy $3 1
  ${EndIf}
  ${If} $3 == 0
    StrCpy $1 0
    ${While} $1 < $2
      StrCpy $4 $0 1 $1
      ${If} $4 == ";"
      ${OrIf} $4 == "*"
      ${OrIf} $4 == "?"
      ${OrIf} $4 == "["
      ${OrIf} $4 == "]"
      ${OrIf} $4 == '"'
        StrCpy $3 1
        ${Break}
      ${EndIf}
      IntOp $1 $1 + 1
    ${EndWhile}
  ${EndIf}
  StrCpy $0 $3
  Pop $4
  Pop $3
  Pop $2
  Pop $1
  Exch $0           ; restore old $0, leave the result (0 ok / 1 bad) on the stack
FunctionEnd

Function DirLeave
  Push $INSTDIR
  Call ValidateDir
  Pop $0
  ${If} $0 != 0
    MessageBox MB_ICONEXCLAMATION|MB_OK "Please choose a normal folder such as C:\Program Files\PathMan.$\r$\n$\r$\nThe folder may not be a drive root and may not contain these characters:  ;  *  ?  [  ]  $\""
    Abort
  ${EndIf}
FunctionEnd

; ── install ──────────────────────────────────────────────────────────────────
Section "PathMan (required)" SecMain
  SectionIn RO
  SetOutPath "$INSTDIR"
  File "${EXE}"
  File "${SRCDIR}\README.md"
  File "${SRCDIR}\LICENSE"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  WriteRegStr   HKLM "${APP_KEY}" "InstallDir" "$INSTDIR"
  WriteRegStr   HKLM "${APP_KEY}" "Version"    "${VERSION}"

  WriteRegStr   HKLM "${UNINST_KEY}" "DisplayName"          "${APPNAME}"
  WriteRegStr   HKLM "${UNINST_KEY}" "DisplayVersion"       "${VERSION}"
  WriteRegStr   HKLM "${UNINST_KEY}" "Publisher"            "${PUBLISHER}"
  WriteRegStr   HKLM "${UNINST_KEY}" "URLInfoAbout"         "${URL}"
  WriteRegStr   HKLM "${UNINST_KEY}" "InstallLocation"      "$INSTDIR"
  WriteRegStr   HKLM "${UNINST_KEY}" "DisplayIcon"          "$INSTDIR\pathman.exe"
  WriteRegStr   HKLM "${UNINST_KEY}" "UninstallString"      '"$INSTDIR\uninstall.exe"'
  WriteRegStr   HKLM "${UNINST_KEY}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoRepair" 1
  SectionGetSize ${SecMain} $0
  WriteRegDWORD HKLM "${UNINST_KEY}" "EstimatedSize" $0
SectionEnd

Section "Add PathMan to the system PATH" SecPath
  DetailPrint "Adding $INSTDIR to the system PATH (an automatic backup is taken first)"
  nsExec::ExecToLog '"$INSTDIR\pathman.exe" --no-color path add "$INSTDIR" --scope system'
  Pop $0
  ; tell running programs (Explorer, new terminals) that the environment changed
  SendMessage ${HWND_BROADCAST} ${WM_WININICHANGE} 0 "STR:Environment" /TIMEOUT=5000
  ${If} $0 != 0
    ; on an upgrade into the same folder the entry already exists: that is fine
    ${If} $PrevDir != $INSTDIR
      DetailPrint "pathman exited with code $0 - PATH was not changed"
      MessageBox MB_ICONEXCLAMATION|MB_OK|MB_SETFOREGROUND "PathMan was installed, but could not be added to PATH (exit code $0).$\r$\n$\r$\nAdd it yourself later with:$\r$\n    pathman path add $\"$INSTDIR$\" --scope system" /SD IDOK
    ${EndIf}
  ${EndIf}
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecMain} "The pathman command-line tool, README and license."
  !insertmacro MUI_DESCRIPTION_TEXT ${SecPath} "Add the install folder to the system PATH so 'pathman' works in every terminal."
!insertmacro MUI_FUNCTION_DESCRIPTION_END

; ── uninstall ────────────────────────────────────────────────────────────────
Function un.onInit
  !if "${ARCH}" != "386"
    SetRegView 64
  !endif
FunctionEnd

Section "Uninstall"
  ; Remove our PATH entry first - this needs pathman.exe, so do it before deleting files.
  ; Harmless if the entry is absent (e.g. the PATH component was not installed).
  IfFileExists "$INSTDIR\pathman.exe" 0 +5
    DetailPrint "Removing $INSTDIR from the system PATH (an automatic backup is taken first)"
    nsExec::ExecToLog '"$INSTDIR\pathman.exe" --no-color path remove "$INSTDIR" --scope system'
    Pop $0
    SendMessage ${HWND_BROADCAST} ${WM_WININICHANGE} 0 "STR:Environment" /TIMEOUT=5000

  Delete "$INSTDIR\pathman.exe"
  Delete "$INSTDIR\README.md"
  Delete "$INSTDIR\LICENSE"
  Delete "$INSTDIR\uninstall.exe"
  RMDir  "$INSTDIR"            ; only removes the folder if it is empty

  DeleteRegKey HKLM "${UNINST_KEY}"
  DeleteRegKey HKLM "${APP_KEY}"
  ; backups in %LOCALAPPDATA%\pathman are intentionally kept
SectionEnd
