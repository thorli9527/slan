import { workspacePanelPath } from '../app-routing';
import { AppComponentData } from '../overview/app.component.data';
import {
  ApiDevice,
  ApiDNSRecord,
  ApiDNSZone,
  ApiSecurityGroup,
  ApiSecurityRule,
  ApiUserAlias,
  ApiWorkspace,
  ApiWorkspaceDevice,
  DeviceExposureRow,
  DeviceGroupRow,
  DeviceRow,
  DNSRow,
  DNSZoneRow,
  MemberRow,
  NavItem,
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

export abstract class AppComponentNetworks extends AppComponentData {
  selectWorkspace(workspace: WorkspaceRow): void {
    this.selectedWorkspaceId = workspace.workspaceId;
    this.editingWorkspaceName = workspace.name;
    this.workspacePanel = 'zones';
  }

  openWorkspaceDetail(workspace: WorkspaceRow, panel: WorkspacePanel = 'zones'): void {
    this.closeInlinePopovers();
    this.closeDeviceExposureDialogState();
    this.selectedWorkspaceId = workspace.workspaceId;
    this.editingWorkspaceName = workspace.name;
    this.workspacePanel = panel;
    this.workspaceRouteMode = 'detail';
    this.active = 'workspaces';
    this.navigateTo(workspacePanelPath(workspace.workspaceId, panel, this.selectedZoneId, this.selectedSecurityGroupId));
    void this.loadWorkspaceDevices(workspace.workspaceId);
    void this.loadWorkspaceResources(workspace.workspaceId);
  }

  backToWorkspaceList(): void {
    this.closeInlinePopovers();
    this.closeDeviceExposureDialogState();
    this.workspaceRouteMode = 'list';
    this.active = 'workspaces';
    this.navigateTo('/spaces');
  }

  openWorkspacePanel(workspace: WorkspaceRow, panel: WorkspacePanel): void {
    this.openWorkspaceDetail(workspace, panel);
  }

  openZoneRecords(zone?: DNSZoneRow): void {
    this.closeInlinePopovers();
    if (zone) {
      this.selectedZoneId = zone.zoneId ?? this.selectedZoneId;
    }
    this.workspacePanel = 'records';
    this.navigateTo(workspacePanelPath(this.selectedWorkspaceId, 'records', this.selectedZoneId, this.selectedSecurityGroupId));
  }

  openSecurityRules(): void {
    this.closeInlinePopovers();
    this.selectedSecurityGroupId = this.currentSecurityGroups[0]?.securityGroupId ?? this.selectedSecurityGroupId;
    this.workspacePanel = 'securityRules';
    this.navigateTo(workspacePanelPath(this.selectedWorkspaceId, 'securityRules', this.selectedZoneId, this.selectedSecurityGroupId));
  }

  openSecurityGroupRules(group: SecurityGroupRow): void {
    this.closeInlinePopovers();
    this.selectedSecurityGroupId = group.securityGroupId;
    this.workspacePanel = 'securityRules';
    this.navigateTo(workspacePanelPath(this.selectedWorkspaceId, 'securityRules', this.selectedZoneId, this.selectedSecurityGroupId));
    void this.loadSecurityRules(group.securityGroupId);
  }

  setWorkspacePanel(panel: WorkspacePanel): void {
    this.closeInlinePopovers();
    this.workspacePanel = panel;
    this.navigateTo(workspacePanelPath(this.selectedWorkspaceId, panel, this.selectedZoneId, this.selectedSecurityGroupId));
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
      this.workspaceTemplateKey = preset.code;
      return;
    }
    const slugValue = slug(this.workspaceName);
    this.workspaceCode = slugValue;
    this.workspaceTemplateKey = slugValue;
  }

  openWorkspaceDialog(): void {
    this.closeInlinePopovers();
    this.workspaceName = '默认网络';
    this.workspaceCode = '';
    this.workspaceTemplateKey = 'custom';
    this.workspaceIntraGroupPolicy = 'allow';
    this.workspaceDefault = false;
    this.workspaceDialogMessage = '';
    this.workspaceDialogMode = 'create';
    this.showWorkspaceDialog = true;
  }

  openEditWorkspaceDialog(workspace: WorkspaceRow): void {
    this.closeInlinePopovers();
    this.selectedWorkspaceId = workspace.workspaceId;
    this.workspaceName = workspace.name;
    this.workspaceCode = workspace.code;
    this.workspaceTemplateKey = workspace.template;
    this.workspaceIntraGroupPolicy = workspace.intraGroupPolicy;
    this.workspaceDefault = workspace.default;
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
      await this.saveWorkspaceDialog();
      return;
    }
    const code = slug(this.workspaceCode || this.workspaceName);
    if (this.isWorkspaceCodeDuplicated(code)) {
      this.workspaceDialogMessage = '当前用户下网络名称不能重复';
      return;
    }
    try {
      await this.api.post(WEB_API.networks(), {
        ownerUserId: this.effectiveUserId,
        actorUserId: this.effectiveUserId,
        name: this.workspaceName,
        code,
        templateKey: this.workspaceTemplateKey || code,
        intraGroupPolicy: this.workspaceIntraGroupPolicy,
        default: this.workspaceDefault,
      });
      await this.loadDashboard(this.currentUserId);
      this.closeWorkspaceDialog();
      return;
    } catch {
      if (!this.isDemoMode) {
        this.workspaceDialogMessage = '创建网络失败';
        this.notifyStateChanged();
        return;
      }
    }
    const id = compactUuid();
    this.workspaces = [
      ...this.workspaces,
      { networkId: id, workspaceId: id, name: this.workspaceName, code, template: this.workspaceTemplateKey || code || 'custom', intraGroupPolicy: this.workspaceIntraGroupPolicy, default: this.workspaceDefault, members: 1, devices: 0, zone: `${code}.${id}.${DEFAULT_USER_ID}.sub.staticlss.com` },
    ];
    this.closeWorkspaceDialog();
  }

  async saveWorkspaceDialog(): Promise<void> {
    const workspace = this.selectedWorkspace;
    if (!workspace || !this.workspaceName.trim()) {
      return;
    }
    const code = slug(this.workspaceCode || this.workspaceName);
    if (this.isWorkspaceCodeDuplicated(code, workspace.workspaceId)) {
      this.workspaceDialogMessage = '当前用户下网络名称不能重复';
      return;
    }
    const name = this.workspaceName.trim();
    try {
      const updated = await this.api.patch<ApiWorkspace>(WEB_API.network(workspace.workspaceId, this.effectiveUserId), {
        actorUserId: this.effectiveUserId,
        name,
        code,
        templateKey: this.workspaceTemplateKey || code,
        intraGroupPolicy: this.workspaceIntraGroupPolicy,
        default: this.workspaceDefault,
      });
      workspace.name = updated.name;
      workspace.code = updated.code ?? code;
      workspace.template = updated.templateKey ?? this.workspaceTemplateKey ?? workspace.code;
      workspace.intraGroupPolicy = updated.intraGroupPolicy === 'deny' ? 'deny' : 'allow';
      workspace.default = !!updated.default;
    } catch {
      if (!this.isDemoMode) {
        this.workspaceDialogMessage = '更新网络失败';
        this.notifyStateChanged();
        return;
      }
      workspace.name = name;
      workspace.code = code;
      workspace.template = this.workspaceTemplateKey || workspace.code;
      workspace.intraGroupPolicy = this.workspaceIntraGroupPolicy;
      workspace.default = this.workspaceDefault;
    }
    workspace.zone = `${slug(workspace.code)}.${workspace.workspaceId}.${this.effectiveUserId}.sub.staticlss.com`;
    this.closeWorkspaceDialog();
  }

  openWorkspaceNameTagDialog(workspace: WorkspaceRow): void {
    this.closeDeviceExposureDialogState();
    if (this.showWorkspaceNameTagDialog && this.editingWorkspace?.workspaceId === workspace.workspaceId) {
      this.closeWorkspaceTagDialogs();
      return;
    }
    this.editingWorkspace = workspace;
    this.workspaceDialogMessage = '';
    this.workspaceNameValue = workspace.name;
    this.showWorkspaceNameTagDialog = true;
    this.showWorkspacePolicyTagDialog = false;
  }

  openWorkspacePolicyTagDialog(workspace: WorkspaceRow): void {
    this.closeDeviceExposureDialogState();
    if (this.showWorkspacePolicyTagDialog && this.editingWorkspace?.workspaceId === workspace.workspaceId) {
      this.closeWorkspaceTagDialogs();
      return;
    }
    this.editingWorkspace = workspace;
    this.workspaceDialogMessage = '';
    this.workspaceIntraGroupPolicyValue = workspace.intraGroupPolicy;
    this.workspaceDefaultValue = workspace.default;
    this.showWorkspaceNameTagDialog = false;
    this.showWorkspacePolicyTagDialog = true;
  }

  closeWorkspaceTagDialogs(): void {
    this.showWorkspaceNameTagDialog = false;
    this.showWorkspacePolicyTagDialog = false;
    this.editingWorkspace = null;
    this.workspaceDialogMessage = '';
  }

  async saveWorkspaceNameTagDialog(): Promise<void> {
    if (!this.editingWorkspace || !this.workspaceNameValue.trim()) {
      return;
    }
    const name = this.workspaceNameValue.trim();
    try {
      const updated = await this.api.patch<ApiWorkspace>(WEB_API.network(this.editingWorkspace.workspaceId, this.effectiveUserId), {
        actorUserId: this.effectiveUserId,
        name,
      });
      this.editingWorkspace.name = updated.name;
      this.editingWorkspace.code = updated.code ?? this.editingWorkspace.code;
      this.editingWorkspace.template = updated.templateKey ?? this.editingWorkspace.template ?? this.editingWorkspace.code;
    } catch {
      if (!this.isDemoMode) {
        this.workspaceDialogMessage = '更新网络名称失败';
        this.notifyStateChanged();
        return;
      }
      this.editingWorkspace.name = name;
      this.editingWorkspace.template = this.editingWorkspace.template || this.editingWorkspace.code;
    }
    this.editingWorkspace.zone = `${slug(this.editingWorkspace.code)}.${this.editingWorkspace.workspaceId}.${this.effectiveUserId}.sub.staticlss.com`;
    this.closeWorkspaceTagDialogs();
  }

  async saveWorkspacePolicyTagDialog(): Promise<void> {
    if (!this.editingWorkspace) {
      return;
    }
    try {
      const updated = await this.api.patch<ApiWorkspace>(WEB_API.network(this.editingWorkspace.workspaceId, this.effectiveUserId), {
        actorUserId: this.effectiveUserId,
        intraGroupPolicy: this.workspaceIntraGroupPolicyValue,
        default: this.workspaceDefaultValue,
      });
      this.editingWorkspace.intraGroupPolicy = updated.intraGroupPolicy === 'deny' ? 'deny' : 'allow';
      this.editingWorkspace.default = !!updated.default;
    } catch {
      if (!this.isDemoMode) {
        this.workspaceDialogMessage = '更新网络策略失败';
        this.notifyStateChanged();
        return;
      }
      this.editingWorkspace.intraGroupPolicy = this.workspaceIntraGroupPolicyValue;
      this.editingWorkspace.default = this.workspaceDefaultValue;
    }
    this.closeWorkspaceTagDialogs();
  }

  async removeWorkspace(workspace: WorkspaceRow): Promise<void> {
    try {
      await this.api.delete(WEB_API.network(workspace.workspaceId, this.effectiveUserId));
    } catch {
      if (!this.isDemoMode) {
        this.workspaceDialogMessage = '删除网络失败';
        this.notifyStateChanged();
        return;
      }
    }

    this.workspaces = this.workspaces.filter((item) => item.workspaceId !== workspace.workspaceId);
    this.dnsZones = this.dnsZones.filter((item) => item.workspaceId !== workspace.workspaceId);
    this.dnsRecords = this.dnsRecords.filter((item) => item.workspaceId !== workspace.workspaceId);
    this.securityGroups = this.securityGroups.filter((item) => item.workspaceId !== workspace.workspaceId);
    this.workspaceDeviceInvites = this.workspaceDeviceInvites.filter((item) => item.workspaceId !== workspace.workspaceId && item.networkId !== workspace.workspaceId);
    this.deviceBootstrapKeys = this.deviceBootstrapKeys.filter((item) => item.networkId !== workspace.workspaceId);
    delete this.workspaceDeviceIdsByWorkspace[workspace.workspaceId];
    delete this.workspaceDeviceGroupsByWorkspace[workspace.workspaceId];
    delete this.workspaceDeviceGroupIdsByDeviceByWorkspace[workspace.workspaceId];
    this.selectedZoneId = '';
    this.selectedSecurityGroupId = '';

    const nextWorkspace = this.workspaces[0];
    this.selectedWorkspaceId = nextWorkspace?.workspaceId ?? '';
    if (!nextWorkspace) {
      this.workspaceRouteMode = 'list';
    }
    this.notifyStateChanged();
  }

  override isWorkspaceCodeDuplicated(code: string, exceptWorkspaceId = ''): boolean {
    return this.workspaces.some((workspace) => workspace.workspaceId !== exceptWorkspaceId && workspace.code === code);
  }

  intraGroupPolicyLabel(policy: string): string {
    return policy === 'deny' ? '组内隔离' : '组内互通';
  }

  async addWorkspaceDeviceGroupReference(): Promise<void> {
    const groupId = this.selectedWorkspaceDeviceGroupReferenceId;
    if (!groupId || !this.selectedWorkspaceId) {
      return;
    }
    this.deviceListMessage = '';
    try {
      await this.api.post(WEB_API.networkDeviceGroups(this.selectedWorkspaceId), {
        groupId,
        actorUserId: this.effectiveUserId,
      });
      this.selectedWorkspaceDeviceGroupReferenceId = '';
      await Promise.all([
        this.loadWorkspaceDeviceGroups(this.selectedWorkspaceId),
        this.loadWorkspaceDevices(this.selectedWorkspaceId),
      ]);
    } catch {
      this.deviceListMessage = '引用设备分组失败';
    }
    this.notifyStateChanged();
  }

  async removeWorkspaceDeviceGroupReference(group: DeviceGroupRow): Promise<void> {
    if (!this.selectedWorkspaceId) {
      return;
    }
    this.deviceListMessage = '';
    try {
      await this.api.delete(WEB_API.networkDeviceGroup(this.selectedWorkspaceId, group.groupId, this.effectiveUserId));
      await Promise.all([
        this.loadWorkspaceDeviceGroups(this.selectedWorkspaceId),
        this.loadWorkspaceDevices(this.selectedWorkspaceId),
      ]);
    } catch {
      this.deviceListMessage = '取消设备分组引用失败';
    }
    this.notifyStateChanged();
  }

}
