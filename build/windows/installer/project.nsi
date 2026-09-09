Unicode true

!include "wails_tools.nsh"

VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_HEADERIMAGE
!define MUI_HEADERIMAGE_RIGHT
!define MUI_HEADERIMAGE_BITMAP "resources\header.bmp"
!define MUI_WELCOMEPAGE_BITMAP "resources\welcome.bmp"
!define MUI_WELCOMEPAGE_BITMAP_STRETCH
!define MUI_HEADERIMAGE_BITMAP_STRETCH
!define MUI_BGCOLOR "F7F7F8"
!define MUI_TEXTCOLOR "171719"
!define MUI_INSTALLCOLORS "171719 F7F7F8"
!define MUI_COMPONENTSPAGE_SMALLDESC
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXECUTABLE}"
!define MUI_FINISHPAGE_RUN_TEXT "立即打开 AI ENV"
!define MUI_FINISHPAGE_LINK "访问官网 www.nsmao.com"
!define MUI_FINISHPAGE_LINK_LOCATION "https://www.nsmao.com"
!define MUI_WELCOMEPAGE_TITLE "欢迎使用 AI ENV"
!define MUI_WELCOMEPAGE_TEXT "统一管理 Claude Code、Claude Desktop、Codex、Antigravity、OpenCode 和 Grok。\r\n\r\n安装程序会保留你的现有配置，并支持自定义安装目录。"
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_ABORTWARNING

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "SimpChinese"

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe"
InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
ShowInstDetails show

Function .onInit
    !insertmacro wails.checkArchitecture

    # Prefer the last selected directory. Older releases did not write
    # InstallLocation, so fall back to the executable recorded as DisplayIcon.
    SetRegView 64
    ReadRegStr $0 HKLM "${UNINST_KEY}" "InstallLocation"
    ${If} $0 == ""
        ReadRegStr $0 HKLM "${UNINST_KEY}" "DisplayIcon"
        ${If} $0 != ""
            ${GetParent} "$0" $0
        ${EndIf}
    ${EndIf}
    ${If} $0 != ""
        StrCpy $INSTDIR "$0"
    ${EndIf}
FunctionEnd

Section "主程序（必需）" SEC_APP
    SectionIn RO

    !insertmacro wails.setShellContext
    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR
    !insertmacro wails.files

    # Remove old shortcuts first so unchecked options also apply on upgrades.
    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols
    !insertmacro wails.writeUninstaller
    WriteRegStr HKLM "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
SectionEnd

Section "创建桌面快捷方式" SEC_DESKTOP_SHORTCUT
    !insertmacro wails.setShellContext
    CreateShortcut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
SectionEnd

Section "创建开始菜单快捷方式" SEC_STARTMENU_SHORTCUT
    !insertmacro wails.setShellContext
    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_APP} "安装 ${INFO_PRODUCTNAME} 主程序和卸载程序。"
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_DESKTOP_SHORTCUT} "在桌面创建 ${INFO_PRODUCTNAME} 快捷方式。"
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_STARTMENU_SHORTCUT} "在开始菜单创建 ${INFO_PRODUCTNAME} 快捷方式。"
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "uninstall"
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}"
    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols
    !insertmacro wails.deleteUninstaller
SectionEnd
