#define MyAppName "SLAN Client V2"
#define MyAppPublisher "SLAN"
#define MyAppExeName "slan_client_v2.exe"
#define MyAppVersion "0.1.0"

#ifndef SourceDir
  #define SourceDir "."
#endif

#ifndef OutputDir
  #define OutputDir "."
#endif

[Setup]
AppId={{77F0F18E-A325-4A23-9983-F4E67133A702}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={localappdata}\Programs\SLAN Client V2
UsePreviousAppDir=no
DisableProgramGroupPage=yes
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir={#OutputDir}
OutputBaseFilename=SLAN-Client-V2-Setup
Compression=lzma
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\{#MyAppExeName}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional icons:"; Flags: unchecked

[Dirs]
Name: "{app}"; Permissions: users-modify

[Files]
Source: "{#SourceDir}\slan_client_v2.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\client-core-helper.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\client-core-service.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\flutter_windows.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\client_core_plugin_plugin.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\data\*"; DestDir: "{app}\data"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{autodesktop}\SLAN Client V2"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "Launch SLAN Client V2"; Flags: nowait postinstall skipifsilent

[Code]
function ExecHidden(const Filename: string; const Params: string; const Wait: TExecWait): Boolean;
var
  ResultCode: Integer;
begin
  Result := Exec(Filename, Params, '', SW_HIDE, Wait, ResultCode) and (ResultCode = 0);
end;

procedure StopExistingRuntime();
begin
  ExecHidden(ExpandConstant('{sys}\taskkill.exe'), '/F /IM slan_client_v2.exe', ewWaitUntilTerminated);
  ExecHidden(ExpandConstant('{sys}\taskkill.exe'), '/F /IM client-core-service.exe', ewWaitUntilTerminated);
  ExecHidden(ExpandConstant('{sys}\taskkill.exe'), '/F /IM client-core-helper.exe', ewWaitUntilTerminated);
end;

procedure ClearPreviousClientV2State();
begin
  DeleteFile(ExpandConstant('{commonappdata}\SLAN\client-v2-session.json'));
  DeleteFile(ExpandConstant('{commonappdata}\SLAN\client-v2-control-tasks.xml'));
end;

procedure ClearPreviousInstallDir();
begin
  DelTree(ExpandConstant('{app}'), True, True, True);
end;

function InitializeSetup(): Boolean;
begin
  StopExistingRuntime();
  ClearPreviousClientV2State();
  Result := True;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssInstall then begin
    ClearPreviousInstallDir();
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then begin
    StopExistingRuntime();
  end;
end;
