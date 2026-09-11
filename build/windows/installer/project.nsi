Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the ProjectInfo file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "MyProject" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "MyCompany" # Default "{{.Info.CompanyName}}"
## !define INFO_PRODUCTNAME    "MyProduct" # Default "{{.Info.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.Info.ProductVersion}}"
## !define INFO_COPYRIGHT      "Copyright" # Default "{{.Info.Copyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
####
## GrabOne installs for all users by default and lets the user switch to a
## per-user install on the "Choose Users" page of the wizard.
##
## The manifest stays at "user" on purpose. The application starts this
## installer itself when it applies an update, and CreateProcess cannot start a
## binary that asks for elevation in its manifest at all: it fails with
## ERROR_ELEVATION_REQUIRED before any prompt is shown. So the installer starts
## unelevated and re-launches itself through ShellExecute("runas") once the
## all-users scope is confirmed (see EnsureElevated).
##
## The scope is decided at run time, so WAILS_INSTALL_SCOPE is deliberately left
## undefined and the registry work below uses SHCTX instead of the compile-time
## wails.setShellContext / wails.writeUninstaller macros.
####
!define REQUEST_EXECUTION_LEVEL "user"
####
## Include the wails tools
####
!include "wails_tools.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"
!include "nsDialogs.nsh"

!define ALLUSERS_INSTALL_DIR    "$PROGRAMFILES64\${INFO_PRODUCTNAME}"
!define CURRENTUSER_INSTALL_DIR "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"

Var InstallForAllUsers     # "1" per-machine, "0" per-user
Var ModeFromCommandLine    # "1" when /ALLUSERS or /CURRENTUSER fixed the scope
Var InstDirFromCommandLine # "1" when /D= gave a directory we must not overwrite
Var PreviousInstallDir     # Location of the install we are replacing, if any
Var PreviousAllUsers       # Scope of that install, "" when there is none
Var ModeRadioAllUsers
Var ModeRadioCurrentUser
Var CloseAttempts          # How often we have asked Windows to end the process
Var GraceWaited            # "1" once we have waited out a self-update handover

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
Page custom InstallModePageCreate InstallModePageLeave # For whom to install page.
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!define MUI_FINISHPAGE_RUN ""
!define MUI_FINISHPAGE_RUN_TEXT "Start GrabOne"
!define MUI_FINISHPAGE_RUN_FUNCTION StartApplication
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
InstallDir "${ALLUSERS_INSTALL_DIR}" # Default folder, replaced in .onInit once the scope is known.
ShowInstDetails show # This will always show the installation details.

####
## Scope helpers
####

# Shared by the installer and the uninstaller: point $SMPROGRAMS, $DESKTOP and
# SHCTX at the machine hive or the user hive to match the install scope.
!macro GRABONE_SHELL_CONTEXT UN
Function ${UN}SetShellContextForMode
    ${If} $InstallForAllUsers == "1"
        SetShellVarContext all
    ${Else}
        SetShellVarContext current
    ${EndIf}
FunctionEnd

# Pushes "1" when this process already holds an administrator token.
Function ${UN}IsElevated
    UserInfo::GetAccountType
    Pop $0
    ${If} $0 == "admin"
        Push "1"
    ${Else}
        Push "0"
    ${EndIf}
FunctionEnd
!macroend
!insertmacro GRABONE_SHELL_CONTEXT ""
!insertmacro GRABONE_SHELL_CONTEXT "un."

# Windows locks a running image against writing, so replacing or deleting
# GrabOne.exe while it is open fails. Left alone NSIS reports that as a bare
# "error opening file for writing", which says nothing about the real cause, so
# check for it up front and offer a way out.
!macro GRABONE_CLOSE_RUNNING UN ASK
Function ${UN}CloseRunningApplication
    StrCpy $CloseAttempts 0
    StrCpy $GraceWaited "0"

    check:
    IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}" 0 done

    # Opening the image for append succeeds only when nothing is running it.
    ClearErrors
    FileOpen $0 "$INSTDIR\${PRODUCT_EXECUTABLE}" a
    ${IfNot} ${Errors}
        FileClose $0
        Goto done
    ${EndIf}

    # When GrabOne updates itself it starts this installer and only then closes,
    # so give the handover a moment before troubling the user about it.
    ${If} $GraceWaited == "0"
        StrCpy $GraceWaited "1"
        Sleep 1500
        Goto check
    ${EndIf}

    # taskkill has had its chances by now, so stop rather than loop on a process
    # that will not end.
    ${If} $CloseAttempts >= 2
        MessageBox MB_OK|MB_ICONSTOP \
            "${INFO_PRODUCTNAME} could not be closed.$\r$\n$\r$\nClose it yourself, then run this again." \
            /SD IDOK
        SetErrorLevel 1
        Abort "${INFO_PRODUCTNAME} is still running."
    ${EndIf}

!if "${ASK}" == "1"
    MessageBox MB_ABORTRETRYIGNORE|MB_ICONEXCLAMATION|MB_DEFBUTTON2 \
        "${INFO_PRODUCTNAME} is still running, so its files cannot be replaced.$\r$\n$\r$\nAbort - stop and change nothing.$\r$\nRetry - close ${INFO_PRODUCTNAME} yourself first, then choose this.$\r$\nIgnore - force ${INFO_PRODUCTNAME} closed now, losing anything in progress." \
        /SD IDIGNORE IDRETRY check IDIGNORE force
    Abort "${INFO_PRODUCTNAME} is still running."
!else
    # Uninstalling removes the application whatever the user says, so close it
    # instead of asking a question with only one useful answer.
    Goto force
!endif

    force:
    IntOp $CloseAttempts $CloseAttempts + 1
    DetailPrint "Closing ${INFO_PRODUCTNAME}"
    nsExec::ExecToStack 'taskkill /F /IM "${PRODUCT_EXECUTABLE}"'
    Pop $0
    Pop $1
    Sleep 2000
    Goto check

    done:
FunctionEnd
!macroend
!insertmacro GRABONE_CLOSE_RUNNING ""    "1" # Installing asks first.
!insertmacro GRABONE_CLOSE_RUNNING "un." "0" # Uninstalling just closes it.

# Pushes "1" when this process may write to Program Files, "2" when an elevated
# copy of the installer has already done the work, "0" when the user refused the
# elevation prompt.
Function EnsureElevated
    Call IsElevated
    Pop $0
    ${If} $0 == "1"
        Push "1"
        Return
    ${EndIf}

    ${GetParameters} $R0

    # Windows only lets the consent dialog take the foreground if the process
    # asking for it holds the foreground itself. Hiding this window first would
    # forfeit that, leaving UAC blinking in the taskbar behind everything, which
    # reads as an installer that did nothing.
    ${IfNot} ${Silent}
        BringToFront
    ${EndIf}

    ClearErrors
    ExecShellWait "runas" "$EXEPATH" "/ALLUSERS $R0"
    ${If} ${Errors}
        ${IfNot} ${Silent}
            BringToFront
        ${EndIf}
        Push "0"
        Return
    ${EndIf}

    Push "2"
FunctionEnd

# Point $INSTDIR at the default folder of the selected scope, unless the command
# line already picked one or we are updating an install that lives elsewhere.
Function ApplyInstallMode
    ${If} $InstDirFromCommandLine == "1"
        Return
    ${EndIf}

    ${If} $PreviousInstallDir != ""
    ${AndIf} $PreviousAllUsers == $InstallForAllUsers
        StrCpy $INSTDIR $PreviousInstallDir
    ${ElseIf} $InstallForAllUsers == "1"
        StrCpy $INSTDIR "${ALLUSERS_INSTALL_DIR}"
    ${Else}
        StrCpy $INSTDIR "${CURRENTUSER_INSTALL_DIR}"
    ${EndIf}
FunctionEnd

# An update has to land on top of the existing install, so start from its scope
# rather than from the all-users default.
Function DetectExistingInstall
    StrCpy $PreviousInstallDir ""
    StrCpy $PreviousAllUsers ""

    SetRegView 64
    ReadRegStr $0 HKLM "${UNINST_KEY}" "InstallLocation"
    ${If} $0 != ""
        StrCpy $PreviousInstallDir $0
        StrCpy $PreviousAllUsers "1"
        StrCpy $InstallForAllUsers "1"
        Return
    ${EndIf}

    ReadRegStr $0 HKCU "${UNINST_KEY}" "InstallLocation"
    ${If} $0 != ""
        StrCpy $PreviousInstallDir $0
        StrCpy $PreviousAllUsers "0"
        StrCpy $InstallForAllUsers "0"
    ${EndIf}
FunctionEnd

Function ReadCommandLineScope
    ${GetParameters} $R0

    ClearErrors
    ${GetOptions} $R0 "/ALLUSERS" $R1
    ${IfNot} ${Errors}
        StrCpy $InstallForAllUsers "1"
        StrCpy $ModeFromCommandLine "1"
    ${EndIf}

    ClearErrors
    ${GetOptions} $R0 "/CURRENTUSER" $R1
    ${IfNot} ${Errors}
        StrCpy $InstallForAllUsers "0"
        StrCpy $ModeFromCommandLine "1"
    ${EndIf}

    ClearErrors
FunctionEnd

####
## Pages
####

Function OnChooseAllUsers
    Pop $0
    ${NSD_Check} $ModeRadioAllUsers
    ${NSD_Uncheck} $ModeRadioCurrentUser
FunctionEnd

Function OnChooseCurrentUser
    Pop $0
    ${NSD_Uncheck} $ModeRadioAllUsers
    ${NSD_Check} $ModeRadioCurrentUser
FunctionEnd

Function InstallModePageCreate
    # The elevated copy of ourselves was given the scope on the command line;
    # asking again would only let the two instances disagree.
    ${If} $ModeFromCommandLine == "1"
        Abort
    ${EndIf}

    !insertmacro MUI_HEADER_TEXT "Choose Users" "Choose who ${INFO_PRODUCTNAME} is installed for."

    nsDialogs::Create 1018
    Pop $0
    ${If} $0 == error
        Abort
    ${EndIf}

    ${NSD_CreateLabel} 0 0 100% 20u "Install ${INFO_PRODUCTNAME} for everyone who uses this computer, or for your account only."
    Pop $1

    ${NSD_CreateRadioButton} 0 28u 100% 12u "Anyone who uses this computer (recommended)"
    Pop $ModeRadioAllUsers
    ${NSD_CreateLabel} 12u 41u 92% 20u "Installs into ${ALLUSERS_INSTALL_DIR}. Windows asks for administrator permission."
    Pop $1

    ${NSD_CreateRadioButton} 0 67u 100% 12u "Only for me"
    Pop $ModeRadioCurrentUser
    ${NSD_CreateLabel} 12u 80u 92% 20u "Installs into ${CURRENTUSER_INSTALL_DIR}. No administrator permission is needed."
    Pop $1

    ${If} $InstallForAllUsers == "1"
        ${NSD_Check} $ModeRadioAllUsers
    ${Else}
        ${NSD_Check} $ModeRadioCurrentUser
    ${EndIf}

    # A label between two radio buttons breaks the grouping Windows would do on
    # its own, so keep them mutually exclusive by hand.
    ${NSD_OnClick} $ModeRadioAllUsers OnChooseAllUsers
    ${NSD_OnClick} $ModeRadioCurrentUser OnChooseCurrentUser

    nsDialogs::Show
FunctionEnd

Function InstallModePageLeave
    ${NSD_GetState} $ModeRadioAllUsers $0
    ${If} $0 == ${BST_CHECKED}
        StrCpy $InstallForAllUsers "1"
    ${Else}
        StrCpy $InstallForAllUsers "0"
    ${EndIf}

    Call ApplyInstallMode

    ${If} $InstallForAllUsers == "1"
        Call EnsureElevated
        Pop $0
        ${If} $0 == "2"
            Quit # The elevated copy ran the whole wizard.
        ${EndIf}
        ${If} $0 == "0"
            MessageBox MB_OK|MB_ICONEXCLAMATION "Installing for all users needs administrator permission.$\r$\n$\r$\nChoose $\"Only for me$\" to install ${INFO_PRODUCTNAME} without it."
            Abort # Stay on this page.
        ${EndIf}
    ${EndIf}
FunctionEnd

# Called from the finish page. When the installer elevated itself the app would
# inherit the administrator token, so let Explorer start it as the logged-in
# user instead.
Function StartApplication
    ClearErrors
    Exec '"$WINDIR\explorer.exe" "$INSTDIR\${PRODUCT_EXECUTABLE}"'
    ${If} ${Errors}
        Exec '"$INSTDIR\${PRODUCT_EXECUTABLE}"'
    ${EndIf}
FunctionEnd

####
## Install
####

Function .onInit
    !insertmacro wails.checkArchitecture

    StrCpy $InstallForAllUsers "1"
    StrCpy $ModeFromCommandLine "0"

    StrCpy $InstDirFromCommandLine "0"
    ${If} $INSTDIR != "${ALLUSERS_INSTALL_DIR}"
        StrCpy $InstDirFromCommandLine "1" # /D= was passed on the command line.
    ${EndIf}

    Call DetectExistingInstall
    Call ReadCommandLineScope

    # A silent run has no page to elevate from. Honour an explicit /ALLUSERS,
    # but fall back to a per-user install rather than failing when the all-users
    # default cannot be met.
    ${If} ${Silent}
    ${AndIf} $InstallForAllUsers == "1"
        Call IsElevated
        Pop $0
        ${If} $0 == "0"
            ${If} $ModeFromCommandLine == "1"
                Call EnsureElevated
                Pop $0
                ${If} $0 == "2"
                    Quit
                ${EndIf}
                ${If} $0 == "0"
                    SetErrorLevel 740 # ERROR_ELEVATION_REQUIRED
                    Abort
                ${EndIf}
            ${Else}
                StrCpy $InstallForAllUsers "0"
            ${EndIf}
        ${EndIf}
    ${EndIf}

    Call ApplyInstallMode
FunctionEnd

Section
    Call SetShellContextForMode
    Call CloseRunningApplication

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    WriteUninstaller "$INSTDIR\uninstall.exe"

    SetRegView 64
    WriteRegStr SHCTX "${UNINST_KEY}" "Publisher" "${INFO_COMPANYNAME}"
    WriteRegStr SHCTX "${UNINST_KEY}" "DisplayName" "${INFO_PRODUCTNAME}"
    WriteRegStr SHCTX "${UNINST_KEY}" "DisplayVersion" "${INFO_PRODUCTVERSION}"
    WriteRegStr SHCTX "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    WriteRegStr SHCTX "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
    WriteRegStr SHCTX "${UNINST_KEY}" "UninstallString" "$\"$INSTDIR\uninstall.exe$\""
    WriteRegStr SHCTX "${UNINST_KEY}" "QuietUninstallString" "$\"$INSTDIR\uninstall.exe$\" /S"
    WriteRegDWORD SHCTX "${UNINST_KEY}" "NoModify" 1
    WriteRegDWORD SHCTX "${UNINST_KEY}" "NoRepair" 1

    ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
    IntFmt $0 "0x%08X" $0
    WriteRegDWORD SHCTX "${UNINST_KEY}" "EstimatedSize" "$0"
SectionEnd

####
## Uninstall
####

Function un.onInit
    SetRegView 64

    # An all-users install registers itself in HKLM under its own location; a
    # per-user one only ever appears in HKCU.
    StrCpy $InstallForAllUsers "0"
    ReadRegStr $0 HKLM "${UNINST_KEY}" "InstallLocation"
    ${If} $0 == $INSTDIR
        StrCpy $InstallForAllUsers "1"
    ${EndIf}

    ${If} $InstallForAllUsers == "1"
        Call un.IsElevated
        Pop $0
        ${If} $0 == "0"
            ${If} ${Silent}
                SetErrorLevel 740 # ERROR_ELEVATION_REQUIRED
                Abort
            ${EndIf}

            # Keep the foreground so the consent dialog can take it, as above.
            BringToFront
            ClearErrors
            ExecShellWait "runas" "$EXEPATH" '_?=$INSTDIR'
            ${If} ${Errors}
                BringToFront
                MessageBox MB_OK|MB_ICONEXCLAMATION "Removing ${INFO_PRODUCTNAME} needs administrator permission."
            ${EndIf}
            Quit
        ${EndIf}
    ${EndIf}

    Call un.SetShellContextForMode
FunctionEnd

Section "uninstall"
    Call un.SetShellContextForMode
    Call un.CloseRunningApplication

    # The WebView2 data folder is always per-user, whatever the install scope is.
    SetShellVarContext current
    RMDir /r "$APPDATA\${PRODUCT_EXECUTABLE}"
    Call un.SetShellContextForMode

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    SetRegView 64
    DeleteRegKey SHCTX "${UNINST_KEY}"

    Delete "$INSTDIR\uninstall.exe"
    RMDir /r $INSTDIR
SectionEnd
