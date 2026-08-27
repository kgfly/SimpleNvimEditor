; Inno Setup script for SimpleNvimEditor.
; Compiled headlessly by package.yml:
;   iscc /DMyAppVersion="1.2.3" /DMyAppArch="amd64" packaging/windows/simplenvim.iss
;
; Unsigned (Phase 1) -- SmartScreen will warn on first run. See the README.

#ifndef MyAppVersion
  #define MyAppVersion "0.0.0"
#endif
#ifndef MyAppArch
  #define MyAppArch "amd64"
#endif

#define MyAppName "SimpleNvimEditor"
#define MyAppExeName "simplenvim.exe"
#define MyAppPublisher "kgfly"
#define MyAppURL "https://github.com/kgfly/SimpleNvimEditor"

[Setup]
AppId={{8F3B1C42-7A5E-4D91-9E2B-6C4A1D8F3E70}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}/issues
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
LicenseFile=..\..\LICENSE
OutputDir=Output
OutputBaseFilename=simplenvim_{#MyAppVersion}_windows_{#MyAppArch}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
SetupIconFile=..\..\src\cmd\simplenvim\icon.ico
ChangesEnvironment=yes
; Per-user install by default so no UAC prompt is needed.
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
#if MyAppArch == "arm64"
ArchitecturesAllowed=arm64
ArchitecturesInstallIn64BitMode=arm64
#else
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
#endif

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Additional shortcuts:"; Flags: unchecked
Name: "addtopath"; Description: "Add SimpleNvimEditor to the &PATH"; GroupDescription: "System integration:"

[Files]
Source: "..\..\stage\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\src\cmd\simplenvim\icon.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\README.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\icon.ico"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\icon.ico"; Tasks: desktopicon

[Registry]
; Always modify the per-user PATH, even for an admin install, to avoid
; HKLM\Environment errors on machines with restrictive policies.
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; \
  ValueData: "{olddata};{app}"; Check: NeedsAddPath(ExpandConstant('{app}')); Tasks: addtopath

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "Launch {#MyAppName}"; Flags: nowait postinstall skipifsilent

[Code]
// Only append to PATH if this exact directory is not already present,
// so repeat installs do not grow the variable without bound.
function NeedsAddPath(Param: string): Boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', OrigPath) then
  begin
    Result := True;
    exit;
  end;
  Result := Pos(';' + Uppercase(Param) + ';', ';' + Uppercase(OrigPath) + ';') = 0;
end;
