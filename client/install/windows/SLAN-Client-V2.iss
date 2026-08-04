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
; Package everything from the staging directory
Source: "{#SourceDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

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

// --- HVCI / Test Signing helpers (must be defined before first use) ---

function IsHvciEnabled(): Boolean;
var
  RegPath: string;
  EnabledVal: Cardinal;
begin
  Result := False;
  RegPath := 'SYSTEM\CurrentControlSet\Control\DeviceGuard\Scenarios\HypervisorEnforcedCodeIntegrity';
  if RegQueryDWordValue(HKLM, RegPath, 'Enabled', EnabledVal) then begin
    Result := (EnabledVal = 1);
  end;
end;

function IsSecureBootEnabled(): Boolean;
var
  RegPath: string;
  EnabledVal: Cardinal;
begin
  Result := False;
  RegPath := 'SYSTEM\CurrentControlSet\Control\SecureBoot\State';
  if RegQueryDWordValue(HKLM, RegPath, 'UEFISecureBootEnabled', EnabledVal) then begin
    Result := (EnabledVal = 1);
  end;
end;

function IsTestSigningEnabled(): Boolean;
var
  ResultCode: Integer;
  OutputFile: string;
  Lines: TArrayOfString;
  I: Integer;
  Line: string;
begin
  Result := False;
  OutputFile := ExpandConstant('{tmp}') + '\\bcdedit-out.txt';
  
  // Use cmd.exe with output redirection (most reliable in Inno Setup)
  Exec(
    ExpandConstant('{sys}\cmd.exe'),
    '/c bcdedit /enum > "' + OutputFile + '" 2>&1',
    '', SW_HIDE, ewWaitUntilTerminated, ResultCode
  );
  
  if LoadStringsFromFile(OutputFile, Lines) then begin
    for I := 0 to GetArrayLength(Lines) - 1 do begin
      Line := Lowercase(Lines[I]);
      if (Pos('testsigning', Line) > 0) and (Pos('yes', Line) > 0) then begin
        Result := True;
        exit;
      end;
    end;
  end;
end;

function EnableTestSigning(): Boolean;
var
  ResultCode: Integer;
begin
  // Method 1: Try bcdedit directly (installer should already be elevated)
  if Exec(ExpandConstant('{sys}\bcdedit.exe'), '/set testsigning on', '', SW_SHOWNORMAL, ewWaitUntilTerminated, ResultCode) and (ResultCode = 0) then begin
    Result := True;
    exit;
  end;
  
  // Method 2: Use PowerShell Start-Process -Verb RunAs for explicit elevation
  if Exec(
    ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "Start-Process bcdedit -ArgumentList ''/set testsigning on'' -Verb RunAs -Wait"',
    '', SW_SHOWNORMAL, ewWaitUntilTerminated, ResultCode
  ) and (ResultCode = 0) then begin
    Result := True;
    exit;
  end;
  
  Result := False;
end;

function StopAndDeleteWindowsService(): Boolean;
var
  ResultCode: Integer;
begin
  // Stop the service first
  Exec(ExpandConstant('{sys}\sc.exe'), 'stop {#ServiceName}', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  // Wait a bit for the service to actually stop
  Sleep(2000);
  // Delete the service
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

  ServiceBinPath := ExpandConstant('{app}\client-core-service.exe') + ' --windows-service';

  // Create the service - binPath value must be quoted as a whole
  Result := Exec(
    ExpandConstant('{sys}\sc.exe'),
    'create {#ServiceName} start= auto obj= LocalSystem DisplayName= "{#ServiceDisplayName}" binPath= "' + ServiceBinPath + '"',
    '',
    SW_HIDE,
    ewWaitUntilTerminated,
    ResultCode
  );
  if not Result or (ResultCode <> 0) then begin
    // If create failed, service might already exist — try to delete and recreate
    StopAndDeleteWindowsService();
    WaitForWindowsServiceDeleted();
    Result := Exec(
      ExpandConstant('{sys}\sc.exe'),
      'create {#ServiceName} start= auto obj= LocalSystem DisplayName= "{#ServiceDisplayName}" binPath= "' + ServiceBinPath + '"',
      '',
      SW_HIDE,
      ewWaitUntilTerminated,
      ResultCode
    ) and (ResultCode = 0);
  end;
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
var
  HvciOn: Boolean;
  TestSigningOn: Boolean;
  SecureBootOn: Boolean;
begin
  if ExecHidden(
    ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "$deadline=(Get-Date).AddSeconds(60); do { $adapter=Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object { $_.Name -eq ''SLAN LAN Adapter'' -or $_.InterfaceDescription -like ''*Wintun*'' -or $_.InterfaceDescription -like ''*WireGuard*Tunnel*'' -or $_.InterfaceDescription -like ''*WireGuardNT*'' } | Select-Object -First 1; if ($adapter -and $adapter.Status -eq ''Up'') { exit 0 }; Start-Sleep -Milliseconds 500 } while ((Get-Date) -lt $deadline); exit 1"',
    ewWaitUntilTerminated
  ) then begin
    exit;
  end;
  // Adapter verification failed — diagnose the cause
  HvciOn := IsHvciEnabled();
  TestSigningOn := IsTestSigningEnabled();
  SecureBootOn := IsSecureBootEnabled();
  if HvciOn and not TestSigningOn then begin
    if SecureBootOn then begin
      RaiseException(
        'Failed to install SLAN Wintun adapter.' + #13 + #10 +
        'Secure Boot is ENABLED — Test Signing cannot be enabled.' + #13 + #10 + #13 + #10 +
        'Please:' + #13 + #10 +
        '  1. Reboot into BIOS/UEFI and disable Secure Boot' + #13 + #10 +
        '  2. In Windows: bcdedit /set testsigning on' + #13 + #10 +
        '  3. Reboot and run the installer again.'
      );
    end else begin
      RaiseException(
        'Failed to install SLAN Wintun adapter. HVCI is enabled but Test Signing is off.' + #13 + #10 +
        'Please run in elevated PowerShell: bcdedit /set testsigning on' + #13 + #10 +
        'Then reboot and run the installer again.'
      );
    end;
  end;
  RaiseException('Failed to install SLAN Wintun adapter.' + #13 + #10 +
    'The adapter was not detected within 30 seconds.' + #13 + #10 + #13 + #10 +
    'Please try:' + #13 + #10 +
    '  1. Run the installer as Administrator (right-click -> Run as administrator)' + #13 + #10 +
    '  2. If it still fails, check Device Manager for any Wintun adapter errors' + #13 + #10 +
    '  3. Reboot and run the installer again.');
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

// --- HVCI-aware adapter check ---

function CheckHvciAndTestSigning(): Boolean;
var
  ResultCode: Integer;
  SecureBootOn: Boolean;
begin
  Result := True;
  if not IsHvciEnabled() then begin
    exit;
  end;
  if IsTestSigningEnabled() then begin
    exit;
  end;
  // HVCI on + Test Signing off — check Secure Boot
  SecureBootOn := IsSecureBootEnabled();
  if SecureBootOn then begin
    MsgBox(
      'SLAN Client requires Test Signing mode to install the Wintun network driver.' + #13 + #10 + #13 + #10 +
      'Your system has Secure Boot ENABLED, which blocks enabling Test Signing.' + #13 + #10 + #13 + #10 +
      'Please do the following manually:' + #13 + #10 +
      '  1. Reboot and enter BIOS/UEFI settings (press F2/Del/F12 during boot)' + #13 + #10 +
      '  2. Find "Secure Boot" and set it to DISABLED' + #13 + #10 +
      '  3. Save and exit BIOS' + #13 + #10 +
      '  4. In Windows, open elevated PowerShell and run:' + #13 + #10 +
      '     bcdedit /set testsigning on' + #13 + #10 +
      '  5. Reboot, then run this installer again.',
      mbInformation, MB_OK
    );
    Result := False;
    exit;
  end;
  // Secure Boot off — try to enable Test Signing automatically
  if MsgBox(
    'SLAN Client requires Test Signing mode to install the Wintun network driver.' + #13 + #10 +
    'Your system has HVCI enabled, which blocks drivers with expired certificates.' + #13 + #10 + #13 + #10 +
    'The installer will enable Test Signing now. A reboot is required.' + #13 + #10 +
    'After rebooting, please run the installer again.',
    mbConfirmation, MB_YESNO
  ) = IDYES then begin
    if EnableTestSigning() then begin
      MsgBox('Test Signing has been enabled. The system will now reboot.' + #13 + #10 +
             'After reboot, please run the SLAN Client installer again.', mbInformation, MB_OK);
      Exec(ExpandConstant('{sys}\shutdown.exe'), '/r /t 5 /c "SLAN Client: rebooting for Test Signing"', '', SW_SHOWNORMAL, ewNoWait, ResultCode);
    end else begin
      MsgBox('Failed to enable Test Signing. Please run in elevated PowerShell:' + #13 + #10 +
             '  bcdedit /set testsigning on' + #13 + #10 +
             'Then reboot and run the installer again.', mbError, MB_OK);
    end;
  end;
  Result := False;
end;

function InitializeSetup(): Boolean;
begin
  StopExistingRuntime();
  DeleteServiceTask();
  DeleteHelperTask();
  Result := CheckHvciAndTestSigning();
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
