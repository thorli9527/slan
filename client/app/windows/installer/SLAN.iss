#define MyAppName "SLAN"
#define MyAppPublisher "SLAN"
#define MyAppExeName "slan_app.exe"
#define MyAppAssocName "SLAN"
#define MyAppVersion "0.1.0"
#define ServiceName "SLANAppCoreService"
#define ServiceDisplayName "SLAN AppCore Service"
#define ServiceHost "127.0.0.1:46391"
#define ServiceControlBaseUrl "http://127.0.0.1:28080"
#define ServiceDriver "wintun"

#ifndef SourceDir
  #define SourceDir "."
#endif

#ifndef OutputDir
  #define OutputDir "."
#endif

[Setup]
AppId={{5D0D5B28-8B2A-4A2A-A6F9-40B530F6D421}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={localappdata}\Programs\SLAN
UsePreviousAppDir=no
DisableProgramGroupPage=yes
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir={#OutputDir}
OutputBaseFilename=SLAN-Setup
Compression=lzma
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\slan_app.exe

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional icons:"; Flags: unchecked

[Files]
Source: "{#SourceDir}\slan_app.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\app-core-helper.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\app-core-service.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\wintun.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\flutter_windows.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\native_assets.json"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\slan_app_core_plugin_windows_plugin.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\data\*"; DestDir: "{app}\data"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{autodesktop}\SLAN"; Filename: "{app}\slan_app.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\slan_app.exe"; Description: "Launch SLAN"; Flags: nowait postinstall skipifsilent

[Code]
function ExecHidden(const Filename: string; const Params: string; const Wait: TExecWait): Boolean;
var
  ResultCode: Integer;
begin
  Result := Exec(
    Filename,
    Params,
    '',
    SW_HIDE,
    Wait,
    ResultCode
  ) and (ResultCode = 0);
end;

function StopAndDeleteWindowsService(): Boolean;
var
  ResultCode: Integer;
begin
  Exec(
    ExpandConstant('{sys}\sc.exe'),
    'stop {#ServiceName}',
    '',
    SW_HIDE,
    ewWaitUntilTerminated,
    ResultCode
  );
  Result := Exec(
    ExpandConstant('{sys}\sc.exe'),
    'delete {#ServiceName}',
    '',
    SW_HIDE,
    ewWaitUntilTerminated,
    ResultCode
  );
end;

procedure StopExistingRuntime();
begin
  ExecHidden(
    ExpandConstant('{sys}\taskkill.exe'),
    '/F /IM slan_app.exe',
    ewWaitUntilTerminated
  );
  ExecHidden(
    ExpandConstant('{sys}\taskkill.exe'),
    '/F /IM app-core-helper.exe',
    ewWaitUntilTerminated
  );
  ExecHidden(
    ExpandConstant('{sys}\taskkill.exe'),
    '/F /IM app-core-service.exe',
    ewWaitUntilTerminated
  );
  StopAndDeleteWindowsService();
end;

procedure ClearPreviousStateData();
begin
  DelTree(ExpandConstant('{commonappdata}\SLAN'), True, True, True);
  DelTree(ExpandConstant('{userappdata}\com.example\slan_app'), True, True, True);
  DelTree(ExpandConstant('{localappdata}\com.example\slan_app'), True, True, True);
end;

procedure ClearPreviousInstallDir();
begin
  DelTree(ExpandConstant('{app}'), True, True, True);
end;

function RegisterAndStartWindowsService(): Boolean;
var
  ResultCode: Integer;
  ServiceBinPath: string;
begin
  ServiceBinPath := '""' + ExpandConstant('{app}\app-core-service.exe') +
    '"" --windows-service --driver {#ServiceDriver} --tcp-host {#ServiceHost} --control-base-url {#ServiceControlBaseUrl}';

  Result := Exec(
    ExpandConstant('{sys}\sc.exe'),
    'create {#ServiceName} start= auto obj= LocalSystem DisplayName= "{#ServiceDisplayName}" binPath= "' + ServiceBinPath + '"',
    '',
    SW_HIDE,
    ewWaitUntilTerminated,
    ResultCode
  ) and (ResultCode = 0);
  if not Result then begin
    exit;
  end;

  Result := Exec(
    ExpandConstant('{sys}\sc.exe'),
    'start {#ServiceName}',
    '',
    SW_HIDE,
    ewWaitUntilTerminated,
    ResultCode
  ) and (ResultCode = 0);
end;

function PrepareDedicatedAdapter(): Boolean;
var
  ResultCode: Integer;
begin
  Result := Exec(
    ExpandConstant('{app}\app-core-service.exe'),
    '--driver {#ServiceDriver} --prepare-adapter',
    '',
    SW_HIDE,
    ewWaitUntilTerminated,
    ResultCode
  ) and (ResultCode = 0);
end;

function ValidateDedicatedAdapterState(): Boolean;
var
  ResultCode: Integer;
  Script: string;
begin
  Script :=
    '$kmtest = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object { $_.InterfaceDescription -like ''*KM-TEST*Loopback Adapter*'' }; ' +
    'if ($kmtest) { exit 11 }; ' +
    'exit 0';
  Result := Exec(
    ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "' + Script + '"',
    '',
    SW_HIDE,
    ewWaitUntilTerminated,
    ResultCode
  ) and (ResultCode = 0);
end;

function InitializeSetup(): Boolean;
begin
  StopExistingRuntime();
  ClearPreviousStateData();
  Result := True;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssInstall then begin
    ClearPreviousInstallDir();
  end;
  if CurStep = ssPostInstall then begin
    if not PrepareDedicatedAdapter() then begin
      RaiseException('Failed to prepare the SLAN Wintun adapter.');
    end;
    if not ValidateDedicatedAdapterState() then begin
      RaiseException('Dedicated SLAN adapter validation failed. KM-TEST may still exist.');
    end;
    if not RegisterAndStartWindowsService() then begin
      RaiseException('Failed to register the SLAN AppCore Service Windows service.');
    end;
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then begin
    StopAndDeleteWindowsService();
  end;
end;
