import { workspacePanelPath } from '../app-routing';
import { AppComponentOverview } from '../overview/app.component.overview';
import {
  ApiDevice,
  ApiDNSRecord,
  ApiDNSZone,
  ApiPublicMapping,
  ApiSecurityGroup,
  ApiSecurityRule,
  ApiUserAlias,
  ApiWorkspace,
  ApiWorkspaceDevice,
  DeviceExposureRow,
  DeviceRow,
  DNSRow,
  DNSZoneRow,
  MemberRow,
  NavItem,
  PublicMappingRow,
  RuleSubjectType,
  SecurityGroupRow,
  SecurityRuleRow,
  SecurityRuleTemplate,
  UserAliasRow,
  WorkspaceDeviceInviteRow,
  WorkspacePanel,
  WorkspaceRow,
} from '../app.models';
import { compactUuid, slug } from '../app.utils';
import { WEB_API } from '../api-paths';
import { DEFAULT_USER_ID } from '../app.seed-data';

export abstract class AppComponentNetworks extends AppComponentOverview {
  setActive(id: string): void {
    this.active = id;
    if (id === 'overview') {
      this.workspaceRouteMode = 'list';
      history.pushState({}, '', '/overview');
      return;
    }
    if (id === 'devices') {
      this.workspaceRouteMode = 'list';
      this.devicePanel = 'list';
      history.pushState({}, '', '/devices');
      return;
    }
    if (id === 'deviceGroups') {
      this.active = 'devices';
      this.workspaceRouteMode = 'list';
      this.devicePanel = 'groups';
      history.pushState({}, '', '/devices/groups');
      return;
    }
    if (id === 'userAliases') {
      this.workspaceRouteMode = 'list';
      history.pushState({}, '', '/user-aliases');
      return;
    }
    if (id === 'workspaces') {
      this.workspaceRouteMode = 'list';
      history.pushState({}, '', '/spaces');
    }
  }

  selectWorkspace(workspace: WorkspaceRow): void {
    this.selectedWorkspaceId = workspace.workspaceId;
    this.editingWorkspaceName = workspace.name;
    this.workspacePanel = 'zones';
  }

  openWorkspaceDetail(workspace: WorkspaceRow, panel: WorkspacePanel = 'zones'): void {
    this.selectedWorkspaceId = workspace.workspaceId;
    this.editingWorkspaceName = workspace.name;
    this.workspacePanel = panel;
    this.workspaceRouteMode = 'detail';
    this.active = 'workspaces';
    history.pushState({}, '', workspacePanelPath(workspace.workspaceId, panel, this.selectedZoneId, this.selectedSecurityGroupId));
    void this.loadWorkspaceDevices(workspace.workspaceId);
  }

  backToWorkspaceList(): void {
    this.workspaceRouteMode = 'list';
    this.active = 'workspaces';
    history.pushState({}, '', '/spaces');
  }

  openWorkspacePanel(workspace: WorkspaceRow, panel: WorkspacePanel): void {
    this.openWorkspaceDetail(workspace, panel);
  }

  openZoneRecords(zone?: DNSZoneRow): void {
    if (zone) {
      this.selectedZoneId = zone.zoneId ?? this.selectedZoneId;
    }
    this.workspacePanel = 'records';
    history.pushState({}, '', workspacePanelPath(this.selectedWorkspaceId, 'records', this.selectedZoneId, this.selectedSecurityGroupId));
  }

  openSecurityRules(): void {
    this.selectedSecurityGroupId = this.currentSecurityGroups[0]?.securityGroupId ?? this.selectedSecurityGroupId;
    this.workspacePanel = 'securityRules';
    history.pushState({}, '', workspacePanelPath(this.selectedWorkspaceId, 'securityRules', this.selectedZoneId, this.selectedSecurityGroupId));
  }

  openSecurityGroupRules(group: SecurityGroupRow): void {
    this.selectedSecurityGroupId = group.securityGroupId;
    this.workspacePanel = 'securityRules';
    history.pushState({}, '', workspacePanelPath(this.selectedWorkspaceId, 'securityRules', this.selectedZoneId, this.selectedSecurityGroupId));
    void this.loadSecurityRules(group.securityGroupId);
  }

  setWorkspacePanel(panel: WorkspacePanel): void {
    if (panel === 'publicMappings' && !this.publicMappingsEnabled) {
      panel = 'zones';
    }
    this.workspacePanel = panel;
    history.pushState({}, '', workspacePanelPath(this.selectedWorkspaceId, panel, this.selectedZoneId, this.selectedSecurityGroupId));
    void this.loadWorkspaceResources(this.selectedWorkspaceId);
  }

  selectWorkspaceForDeviceManagement(workspaceId: string): void {
    this.selectedWorkspaceId = workspaceId;
    this.workspacePanel = 'zones';
    void this.loadWorkspaceDevices(workspaceId);
  }

  onWorkspaceNameChanged(): void {
    const preset = this.workspacePresets.find((item) => item.name === this.workspaceName.trim());
    if (preset) {
      this.workspaceCode = preset.code;
      return;
    }
    this.workspaceCode = slug(this.workspaceName);
  }

  openWorkspaceDialog(): void {
    this.workspaceName = '默认网络';
    this.workspaceCode = '';
    this.workspaceIntraGroupPolicy = 'allow';
    this.workspaceDialogMessage = '';
    this.workspaceDialogMode = 'create';
    this.showWorkspaceDialog = true;
  }

  openEditWorkspaceDialog(workspace: WorkspaceRow): void {
    this.selectedWorkspaceId = workspace.workspaceId;
    this.workspaceName = workspace.name;
    this.workspaceCode = workspace.code;
    this.workspaceIntraGroupPolicy = workspace.intraGroupPolicy;
    this.workspaceDialogMessage = '';
    this.workspaceDialogMode = 'edit';
    this.showWorkspaceDialog = true;
  }

  closeWorkspaceDialog(): void {
    this.showWorkspaceDialog = false;
  }

  async addWorkspace(): Promise<void> {
    this.workspaceDialogMessage = '';
    if (this.workspaceDialogMode === 'edit') {
      this.saveWorkspaceDialog();
      return;
    }
    const code = slug(this.workspaceName);
    if (this.isWorkspaceCodeDuplicated(code)) {
      this.workspaceDialogMessage = '当前用户下网络名称不能重复';
      return;
    }
    try {
      await this.api.post(WEB_API.networks(), {
        ownerUserId: this.currentUserId || DEFAULT_USER_ID,
        name: this.workspaceName,
        code,
        templateKey: code,
        intraGroupPolicy: this.workspaceIntraGroupPolicy,
      });
      await this.loadDashboard(this.currentUserId);
      this.closeWorkspaceDialog();
      return;
    } catch {
      // Keep local template behavior available for UI review.
    }
    const id = compactUuid();
    this.workspaces = [
      ...this.workspaces,
      { networkId: id, workspaceId: id, name: this.workspaceName, code, template: code || 'custom', intraGroupPolicy: this.workspaceIntraGroupPolicy, members: 1, devices: 0, zone: `${code}.${id}.${DEFAULT_USER_ID}.sub.staticlss.com` },
    ];
    this.closeWorkspaceDialog();
  }

  saveWorkspaceDialog(): void {
    const workspace = this.selectedWorkspace;
    if (!workspace || !this.workspaceName.trim()) {
      return;
    }
    const code = workspace.code || slug(this.workspaceName);
    if (this.isWorkspaceCodeDuplicated(code, workspace.workspaceId)) {
      this.workspaceDialogMessage = '当前用户下网络名称不能重复';
      return;
    }
    workspace.name = this.workspaceName.trim();
    workspace.code = code;
    workspace.template = workspace.code;
    workspace.intraGroupPolicy = this.workspaceIntraGroupPolicy;
    workspace.zone = `${slug(workspace.code)}.${workspace.workspaceId}.${this.currentUserId || DEFAULT_USER_ID}.sub.staticlss.com`;
    this.closeWorkspaceDialog();
  }

  openWorkspaceNameTagDialog(workspace: WorkspaceRow): void {
    if (this.showWorkspaceNameTagDialog && this.editingWorkspace?.workspaceId === workspace.workspaceId) {
      this.closeWorkspaceTagDialogs();
      return;
    }
    this.editingWorkspace = workspace;
    this.workspaceNameValue = workspace.name;
    this.showWorkspaceNameTagDialog = true;
    this.showWorkspacePolicyTagDialog = false;
  }

  openWorkspacePolicyTagDialog(workspace: WorkspaceRow): void {
    if (this.showWorkspacePolicyTagDialog && this.editingWorkspace?.workspaceId === workspace.workspaceId) {
      this.closeWorkspaceTagDialogs();
      return;
    }
    this.editingWorkspace = workspace;
    this.workspaceIntraGroupPolicyValue = workspace.intraGroupPolicy;
    this.showWorkspaceNameTagDialog = false;
    this.showWorkspacePolicyTagDialog = true;
  }

  closeWorkspaceTagDialogs(): void {
    this.showWorkspaceNameTagDialog = false;
    this.showWorkspacePolicyTagDialog = false;
    this.editingWorkspace = null;
  }

  async saveWorkspaceNameTagDialog(): Promise<void> {
    if (!this.editingWorkspace || !this.workspaceNameValue.trim()) {
      return;
    }
    const name = this.workspaceNameValue.trim();
    try {
      const updated = await this.api.patch<ApiWorkspace>(WEB_API.network(this.editingWorkspace.workspaceId), {
        name,
      });
      this.editingWorkspace.name = updated.name;
      this.editingWorkspace.code = updated.code ?? this.editingWorkspace.code;
      this.editingWorkspace.template = updated.templateKey ?? this.editingWorkspace.code;
    } catch {
      this.editingWorkspace.name = name;
      this.editingWorkspace.template = this.editingWorkspace.code;
    }
    this.editingWorkspace.zone = `${slug(this.editingWorkspace.code)}.${this.editingWorkspace.workspaceId}.${this.currentUserId || DEFAULT_USER_ID}.sub.staticlss.com`;
    this.closeWorkspaceTagDialogs();
  }

  async saveWorkspacePolicyTagDialog(): Promise<void> {
    if (!this.editingWorkspace) {
      return;
    }
    try {
      const updated = await this.api.patch<ApiWorkspace>(WEB_API.network(this.editingWorkspace.workspaceId), {
        intraGroupPolicy: this.workspaceIntraGroupPolicyValue,
      });
      this.editingWorkspace.intraGroupPolicy = updated.intraGroupPolicy === 'deny' ? 'deny' : 'allow';
    } catch {
      this.editingWorkspace.intraGroupPolicy = this.workspaceIntraGroupPolicyValue;
    }
    this.closeWorkspaceTagDialogs();
  }

  override isWorkspaceCodeDuplicated(code: string, exceptWorkspaceId = ''): boolean {
    return this.workspaces.some((workspace) => workspace.workspaceId !== exceptWorkspaceId && workspace.code === code);
  }

  intraGroupPolicyLabel(policy: string): string {
    return policy === 'deny' ? '组内隔离' : '组内互通';
  }

  async removeWorkspaceDevice(device: DeviceRow): Promise<void> {
    try {
      await this.api.delete(WEB_API.networkDevice(this.selectedWorkspaceId, device.deviceId));
    } catch {
      // Local preview mode removes the row below.
    }
    const currentIds = this.currentWorkspaceDeviceIds().filter((deviceId) => deviceId !== device.deviceId);
    this.workspaceDeviceIdsByWorkspace[this.selectedWorkspaceId] = currentIds;
    this.selectedWorkspace.devices = currentIds.length;
  }

  openWorkspaceDeviceDialog(): void {
    this.bindDeviceQuery = '';
    this.selectedWorkspaceDeviceId = this.queriedBindableWorkspaceDevices[0]?.deviceId ?? '';
    this.showWorkspaceDeviceDialog = true;
  }

  queryWorkspaceDevices(): void {
    const first = this.queriedBindableWorkspaceDevices[0]?.deviceId ?? '';
    if (!this.queriedBindableWorkspaceDevices.some((device) => device.deviceId === this.selectedWorkspaceDeviceId)) {
      this.selectedWorkspaceDeviceId = first;
    }
  }

  closeWorkspaceDeviceDialog(): void {
    this.showWorkspaceDeviceDialog = false;
  }

  async saveWorkspaceDeviceDialog(): Promise<void> {
    const currentIds = this.currentWorkspaceDeviceIds();
    if (!this.selectedWorkspaceDeviceId || currentIds.includes(this.selectedWorkspaceDeviceId)) {
      return;
    }
    const selected = this.devices.find((device) => device.deviceId === this.selectedWorkspaceDeviceId);
    try {
      await this.api.post(WEB_API.networkDevices(this.selectedWorkspaceId), {
        deviceId: this.selectedWorkspaceDeviceId,
        actorUserId: this.currentUserId || selected?.owner || DEFAULT_USER_ID,
        alias: selected?.alias ?? '',
        enabled: true,
      });
      await this.loadWorkspaceDevices(this.selectedWorkspaceId);
      this.closeWorkspaceDeviceDialog();
      this.notifyStateChanged();
      return;
    } catch {
      // Local preview mode updates the in-memory relationship below.
    }
    const nextIds = [...currentIds, this.selectedWorkspaceDeviceId];
    this.workspaceDeviceIdsByWorkspace[this.selectedWorkspaceId] = nextIds;
    this.workspaceDeviceJoinMethods[`${this.selectedWorkspaceId}|${this.selectedWorkspaceDeviceId}`] = '手动添加';
    this.selectedWorkspace.devices = nextIds.length;
    this.closeWorkspaceDeviceDialog();
    this.notifyStateChanged();
  }

  openWorkspaceDeviceAliasDialog(device: DeviceRow): void {
    if (this.showWorkspaceDeviceAliasDialog && this.editingWorkspaceDevice?.deviceId === device.deviceId) {
      this.closeWorkspaceDeviceAliasDialog();
      return;
    }
    this.editingWorkspaceDevice = device;
    this.workspaceDeviceAliasValue = device.alias;
    this.showWorkspaceDeviceAliasDialog = true;
  }

  closeWorkspaceDeviceAliasDialog(): void {
    this.showWorkspaceDeviceAliasDialog = false;
    this.editingWorkspaceDevice = null;
  }

  async saveWorkspaceDeviceAliasDialog(): Promise<void> {
    if (!this.editingWorkspaceDevice || !this.workspaceDeviceAliasValue.trim()) {
      return;
    }
    const device = this.editingWorkspaceDevice;
    const alias = this.workspaceDeviceAliasValue.trim();
    try {
      await this.api.patch(WEB_API.networkDevice(this.selectedWorkspaceId, device.deviceId), {
        alias,
      });
    } catch {
      // Local preview mode updates the current mock row.
    }
    this.devices = this.devices.map((item) => item.deviceId === device.deviceId ? { ...item, alias } : item);
    this.closeWorkspaceDeviceAliasDialog();
  }
}
