#define MyAppName "SLAN Client V2"
#define MyAppPublisher "SLAN"
#define MyAppExeName "slan_client_v2.exe"
#define HelperTaskName "SLAN Client V2 Helper"
#define ServiceTaskName "SLAN Client V2 Service"
#define ServiceName "SLANClientV2Service"
#define ServiceDisplayName "SLAN Client V2 Service"

#ifndef MyAppVersion
  #define MyAppVersion "0.1.0"
#endif

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
Source: "{#SourceDir}\client-core-service.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\wintun.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\flutter_windows.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\client_core_plugin_plugin.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\data\*"; DestDir: "{app}\data"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "{#SourceDir}\tools\*"; DestDir: "{app}\tools"; Flags: ignoreversion recursesubdirs createallsubdirs skipifsourcedoesntexist

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

function StopAndDeleteWindowsService(): Boolean;
var
  ResultCode: Integer;
begin
  Exec(ExpandConstant('{sys}\sc.exe'), 'stop {#ServiceName}', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Exec(ExpandConstant('{sys}\sc.exe'), 'delete {#ServiceName}', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Result := True;
end;

function WindowsServiceExists(): Boolean;
var
  ResultCode: Integer;
begin
  Result := Exec(ExpandConstant('{sys}\sc.exe'), 'query {#ServiceName}', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) and (ResultCode = 0);
end;

function WaitForWindowsServiceDeleted(): Boolean;
var
  Attempts: Integer;
begin
  Result := False;
  for Attempts := 1 to 30 do begin
    if not WindowsServiceExists() then begin
      Result := True;
      exit;
    end;
    Sleep(1000);
  end;
end;

procedure StopExistingRuntime();
begin
  ExecHidden(ExpandConstant('{sys}\taskkill.exe'), '/F /IM slan_client_v2.exe', ewWaitUntilTerminated);
  ExecHidden(ExpandConstant('{sys}\taskkill.exe'), '/F /IM client-core-service.exe', ewWaitUntilTerminated);
  ExecHidden(ExpandConstant('{sys}\taskkill.exe'), '/F /IM client-core-helper.exe', ewWaitUntilTerminated);
  StopAndDeleteWindowsService();
  WaitForWindowsServiceDeleted();
end;

procedure DeleteHelperTask();
begin
  ExecHidden(ExpandConstant('{sys}\schtasks.exe'), '/End /TN "{#HelperTaskName}"', ewWaitUntilTerminated);
  ExecHidden(ExpandConstant('{sys}\schtasks.exe'), '/Delete /TN "{#HelperTaskName}" /F', ewWaitUntilTerminated);
end;

procedure DeleteServiceTask();
begin
  ExecHidden(ExpandConstant('{sys}\schtasks.exe'), '/End /TN "{#ServiceTaskName}"', ewWaitUntilTerminated);
  ExecHidden(ExpandConstant('{sys}\schtasks.exe'), '/Delete /TN "{#ServiceTaskName}" /F', ewWaitUntilTerminated);
end;

procedure ClearPreviousClientV2State();
begin
  DeleteFile(ExpandConstant('{commonappdata}\SLAN\config.json'));
  DeleteFile(ExpandConstant('{commonappdata}\SLAN\client-v2-session.json'));
  DeleteFile(ExpandConstant('{commonappdata}\SLAN\client-v2-control-tasks.xml'));
end;

procedure ClearClientV2State();
var
  StateDir: string;
begin
  StateDir := ExpandConstant('{commonappdata}\SLAN');
  DeleteFile(StateDir + '\config.json');
  DeleteFile(StateDir + '\client-v2-session.json');
  DeleteFile(StateDir + '\client-v2-control-tasks.xml');
  DeleteFile(StateDir + '\client-v2-device-id.txt');
  DeleteFile(StateDir + '\client-v2-network-state.json');
  DeleteFile(StateDir + '\client-v2-assigned-ip.txt');
  DeleteFile(StateDir + '\client-v2-relay-stats.json');
  DeleteFile(StateDir + '\client-v2-relay-policy.json');
  DeleteFile(StateDir + '\mqtt-inbox.xml');
  DelTree(StateDir + '\diagnostics', True, True, True);
  RemoveDir(StateDir);
end;

procedure ClearPreviousInstallDir();
begin
  DelTree(ExpandConstant('{app}'), True, True, True);
end;

function PrepareDedicatedAdapter(): Boolean;
var
  ResultCode: Integer;
begin
  Result := Exec(
    ExpandConstant('{app}\client-core-service.exe'),
    '--prepare-adapter',
    '',
    SW_HIDE,
    ewWaitUntilTerminated,
    ResultCode
  ) and (ResultCode = 0);
end;

function EnsureStableDeviceId(): Boolean;
var
  ResultCode: Integer;
begin
  Result := Exec(
    ExpandConstant('{app}\client-core-service.exe'),
    '--ensure-device-id',
    '',
    SW_HIDE,
    ewWaitUntilTerminated,
    ResultCode
  ) and (ResultCode = 0);
end;

function RegisterAndStartWindowsService(): Boolean;
var
  ResultCode: Integer;
  ServiceBinPath: string;
begin
  if WindowsServiceExists() then begin
    StopAndDeleteWindowsService();
    if not WaitForWindowsServiceDeleted() then begin
      exit;
    end;
  end;

  ServiceBinPath := '""' + ExpandConstant('{app}\client-core-service.exe') + '"" --windows-service';

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

procedure VerifyWintunAdapterInstalled();
begin
  if not ExecHidden(
    ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "$deadline=(Get-Date).AddSeconds(30); do { $adapter=Get-NetAdapter -IncludeHidden -Name ''SLAN LAN Adapter'' -ErrorAction SilentlyContinue; if ($adapter) { Enable-NetAdapter -Name ''SLAN LAN Adapter'' -Confirm:$false -ErrorAction SilentlyContinue | Out-Null; $adapter=Get-NetAdapter -IncludeHidden -Name ''SLAN LAN Adapter'' -ErrorAction SilentlyContinue; if ($adapter -and $adapter.AdminStatus -eq ''Up'') { exit 0 } }; Start-Sleep -Milliseconds 500 } while ((Get-Date) -lt $deadline); exit 1"',
    ewWaitUntilTerminated
  ) then begin
    RaiseException('Failed to install SLAN Wintun adapter. Please allow administrator permission and reinstall.');
  end;
end;

procedure ClearClientV2AppData();
begin
  DelTree(ExpandConstant('{userappdata}\slan_client_v2'), True, True, True);
  DelTree(ExpandConstant('{localappdata}\slan_client_v2'), True, True, True);
  DelTree(ExpandConstant('{userappdata}\SLAN Client V2'), True, True, True);
  DelTree(ExpandConstant('{localappdata}\SLAN Client V2'), True, True, True);
end;

procedure ClearClientV2InstallArtifacts();
begin
  DeleteFile(ExpandConstant('{autodesktop}\SLAN Client V2.lnk'));
  DelTree(ExpandConstant('{app}'), True, True, True);
end;

procedure RemoveDedicatedAdapter();
begin
  ExecHidden(
    ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "$adapter=Get-PnpDevice -Class Net -ErrorAction SilentlyContinue | Where-Object { $_.FriendlyName -eq ''SLAN LAN Adapter'' } | Select-Object -First 1; if ($adapter) { pnputil.exe /remove-device $adapter.InstanceId | Out-Null }"',
    ewWaitUntilTerminated
  );
end;

function InitializeSetup(): Boolean;
begin
  StopExistingRuntime();
  DeleteServiceTask();
  DeleteHelperTask();
  Result := True;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssInstall then begin
    ClearPreviousInstallDir();
  end;
  if CurStep = ssPostInstall then begin
    if not EnsureStableDeviceId() then begin
      RaiseException('Failed to initialize the SLAN device identity.');
    end;
    if not PrepareDedicatedAdapter() then begin
      RaiseException('Failed to prepare the SLAN Wintun adapter.');
    end;
    if not RegisterAndStartWindowsService() then begin
      RaiseException('Failed to register the SLAN Client V2 Windows service.');
    end;
    VerifyWintunAdapterInstalled();
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then begin
    StopExistingRuntime();
    StopAndDeleteWindowsService();
    DeleteServiceTask();
    DeleteHelperTask();
    RemoveDedicatedAdapter();
  end;
  if CurUninstallStep = usPostUninstall then begin
    ClearClientV2State();
    ClearClientV2AppData();
    ClearClientV2InstallArtifacts();
  end;
end;
