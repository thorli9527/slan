import { AppComponentDns } from './app.component.dns';
import {
  ApiDevice,
  ApiDNSRecord,
  ApiDNSZone,
  ApiSecurityGroup,
  ApiSecurityRule,
  ApiWorkspace,
  ApiWorkspaceDevice,
  DeviceExposureRow,
  DeviceRow,
  DNSRow,
  DNSZoneRow,
  MemberRow,
  NavItem,
  RuleSubjectType,
  SecurityGroupRow,
  SecurityRuleRow,
  SecurityRuleTemplate,
  WorkspaceDeviceInviteRow,
  WorkspacePanel,
  WorkspaceRow,
} from '../app.models';
import { WEB_API } from '../api-paths';
import { compactUuid } from '../app.utils';
import { ApiHttpError } from '../app-api.service';


export abstract class AppComponentSecurity extends AppComponentDns {
  async removeSecurityGroup(group: SecurityGroupRow): Promise<void> {
    try {
      await this.api.delete(WEB_API.securityGroup(group.workspaceId, group.securityGroupId, this.effectiveUserId));
      await this.refreshCurrentSecurityGroups();
    } catch {
      if (!this.isDemoMode) {
        this.securityGroupDialogMessage = '删除安全组失败';
        this.notifyStateChanged();
        return;
      }
      this.securityGroups = this.securityGroups.filter((item) => item.securityGroupId !== group.securityGroupId);
    }
    if (this.selectedSecurityGroupId === group.securityGroupId) {
      this.selectedSecurityGroupId = this.currentSecurityGroups[0]?.securityGroupId ?? '';
    }
    this.notifyStateChanged();
  }

  async openSecurityGroupDialog(): Promise<void> {
    this.closeInlinePopovers();
    this.securityGroupDialogMessage = '';
    try {
      const created = await this.api.post<ApiSecurityGroup>(WEB_API.securityGroups(this.selectedWorkspaceId), {
        actorUserId: this.effectiveUserId,
        name: '',
        description: '',
      });
      this.selectedSecurityGroupId = created.securityGroupId;
      await this.refreshCurrentSecurityGroups();
    } catch {
      if (!this.isDemoMode) {
        this.securityGroupDialogMessage = '创建安全组失败';
        this.notifyStateChanged();
        return;
      }
      const now = Math.floor(Date.now() / 1000);
      const group: SecurityGroupRow = {
        securityGroupId: compactUuid(),
        networkId: this.selectedWorkspaceId,
        workspaceId: this.selectedWorkspaceId,
        name: '',
        description: '',
        createdAt: now,
      };
      this.securityGroups = [
        ...this.securityGroups,
        group,
      ];
      this.selectedSecurityGroupId = group.securityGroupId;
    }
    this.notifyStateChanged();
  }

  openSecurityGroupNameTagDialog(group: SecurityGroupRow): void {
    this.closeInlinePopovers();
    if (this.showSecurityGroupNameTagDialog && this.editingSecurityGroup?.securityGroupId === group.securityGroupId) {
      this.closeSecurityGroupNameTagDialog();
      return;
    }
    this.editingSecurityGroup = group;
    this.securityGroupDialogMessage = '';
    this.securityGroupNameValue = group.name;
    this.securityGroupDescriptionValue = group.description || '';
    this.showSecurityGroupNameTagDialog = true;
  }

  closeSecurityGroupNameTagDialog(): void {
    this.showSecurityGroupNameTagDialog = false;
    this.editingSecurityGroup = null;
    this.securityGroupDialogMessage = '';
  }

  async saveSecurityGroupNameTagDialog(): Promise<void> {
    if (!this.editingSecurityGroup) {
      return;
    }
    this.securityGroupDialogMessage = '';
    const group = this.editingSecurityGroup;
    const name = this.securityGroupNameValue.trim();
    const description = this.securityGroupDescriptionValue.trim();
    try {
      const updated = await this.api.patch<ApiSecurityGroup>(WEB_API.securityGroup(group.workspaceId, group.securityGroupId, this.effectiveUserId), {
        actorUserId: this.effectiveUserId,
        name,
        description,
      });
      Object.assign(group, this.mapSecurityGroup(updated));
    } catch {
      if (!this.isDemoMode) {
        this.securityGroupDialogMessage = '更新安全组失败';
        this.notifyStateChanged();
        return;
      }
      group.name = name;
      group.description = description;
    }
    this.closeSecurityGroupNameTagDialog();
    this.notifyStateChanged();
  }

  securityGroupDisplayName(group: SecurityGroupRow): string {
    return group.name.trim() || '--';
  }

  private async refreshCurrentSecurityGroups(): Promise<void> {
    const groups = await this.api.get<{ items: ApiSecurityGroup[] }>(WEB_API.securityGroups(this.selectedWorkspaceId));
    this.securityGroups = [
      ...this.securityGroups.filter((group) => group.workspaceId !== this.selectedWorkspaceId),
      ...(groups.items ?? []).map((group) => this.mapSecurityGroup(group)),
    ];
  }

  openRuleDialog(direction: string): void {
    this.closeInlinePopovers();
    this.securityRuleDialogMessage = '';
    this.ruleDirection = direction;
    this.selectedRuleTemplate = direction === 'egress' ? '全部出站' : 'Web 服务';
    this.applyRuleTemplate(direction);
    this.ruleEnabled = true;
    this.editingRule = null;
    this.ruleDialogMode = 'create';
    this.showIngressRuleDialog = direction === 'ingress';
    this.showEgressRuleDialog = direction === 'egress';
  }

  openEditRuleDialog(rule: SecurityRuleRow): void {
    this.closeInlinePopovers();
    this.securityRuleDialogMessage = '';
    this.ruleDirection = rule.direction;
    this.rulePriority = rule.priority;
    this.ruleAction = rule.action;
    this.ruleProtocol = rule.protocol;
    this.rulePort = rule.port;
    this.ruleDescription = rule.description || '';
    this.ruleEnabled = rule.enabled ?? true;
    this.ruleSubjectType =
      rule.subjectType === 'user' && rule.subjectValue === this.effectiveUserId
        ? 'all'
        : rule.subjectType;
    this.ruleSubjectValue = rule.subjectValue;
    this.selectedRuleTemplate = '自定义';
    this.editingRule = rule;
    this.ruleDialogMode = 'edit';
    this.showIngressRuleDialog = rule.direction === 'ingress';
    this.showEgressRuleDialog = rule.direction === 'egress';
  }

  selectRuleTemplate(direction: string, templateName: string): void {
    this.selectedRuleTemplate = templateName;
    this.applyRuleTemplate(direction);
  }

  applyRuleTemplate(direction = this.ruleDirection): void {
    const template = this.securityRuleTemplates.find((item) => item.name === this.selectedRuleTemplate && item.direction === direction);
    if (!template) {
      this.ruleDirection = direction;
      return;
    }
    this.ruleDirection = direction;
    this.rulePriority = template.priority;
    this.ruleAction = template.action;
    this.ruleProtocol = template.protocol;
    this.rulePort = template.port;
    this.ruleDescription = template.description || '';
    this.ruleSubjectType = 'device_group';
    this.ruleSubjectValue = '';
    this.onRuleSubjectTypeChanged();
  }

  ruleSubjectLabel(rule: SecurityRuleRow): string {
    if (this.isCurrentUserAllDevicesRule(rule)) {
      return '全部设备';
    }
    if (rule.subjectType === 'device') {
      const device = this.devices.find((item) => item.deviceId === rule.subjectValue);
      return device?.alias || rule.subjectValue;
    }
    if (rule.subjectType === 'device_group') {
      const group = this.deviceGroups.find((item) => item.groupId === rule.subjectValue);
      return group?.name || rule.subjectValue;
    }
    if (rule.subjectType === 'user') {
      const member = this.members.find((item) => item.user === rule.subjectValue);
      return member ? `${member.alias} / ${this.userLabel(member.user)}` : rule.subjectValue;
    }
    if (rule.subjectType === 'workspace') {
      const workspace = rule.subjectValue === 'self' ? this.selectedWorkspace : this.workspaces.find((item) => item.workspaceId === rule.subjectValue);
      return workspace?.name ?? rule.subjectValue;
    }
    return rule.subjectValue;
  }

  subjectTypeLabel(type: RuleSubjectType): string {
    return ({ device: '设备', device_group: '设备分组', user: '指定用户', network: '网络', workspace: '网络', all: '全部设备' } as Record<RuleSubjectType, string>)[type];
  }

  onRuleSubjectTypeChanged(): void {
    const first = this.ruleSubjectOptions[0]?.value;
    if (first) {
      this.ruleSubjectValue = first;
      return;
    }
    this.ruleSubjectValue = '';
  }

  private normalizedRulePeer(): { peerType: RuleSubjectType; peerValue: string } {
    return {
      peerType: this.ruleSubjectType === 'device' ? 'device' : 'device_group',
      peerValue: this.ruleSubjectValue,
    };
  }

  private isCurrentUserAllDevicesRule(rule: SecurityRuleRow): boolean {
    return rule.subjectType === 'user' && rule.subjectValue === this.effectiveUserId;
  }

  closeRuleDialog(): void {
    this.showIngressRuleDialog = false;
    this.showEgressRuleDialog = false;
    this.securityRuleDialogMessage = '';
  }

  async saveRuleDialog(): Promise<void> {
    this.securityRuleDialogMessage = '';
    const portFrom = this.rulePort === 'all' ? 0 : Number.parseInt(this.rulePort.split(',')[0], 10) || 0;
    const portTo = this.rulePort === 'all' ? 0 : Number.parseInt(this.rulePort.split(',').at(-1) ?? this.rulePort, 10) || portFrom;
    const peer = this.normalizedRulePeer();
    if (this.ruleDialogMode === 'edit' && this.editingRule) {
      const ruleId = this.editingRule.ruleId?.trim() ?? '';
      if (!ruleId) {
        this.securityRuleDialogMessage = '更新安全规则失败：规则不存在，请刷新页面后重试';
        this.notifyStateChanged();
        return;
      }
      try {
        const updated = await this.api.patch<ApiSecurityRule>(WEB_API.securityRule(ruleId, this.effectiveUserId), {
          actorUserId: this.effectiveUserId,
          direction: this.ruleDirection,
          priority: this.rulePriority,
          action: this.ruleAction,
          protocol: this.ruleProtocol,
          portFrom,
          portTo,
          peerType: peer.peerType,
          peerValue: peer.peerValue,
          description: this.ruleDescription,
          enabled: this.ruleEnabled,
        });
        Object.assign(this.editingRule, this.mapSecurityRule(updated));
      } catch (error) {
        if (!this.isDemoMode) {
          this.securityRuleDialogMessage = this.securityRuleFailureMessage('更新', error);
          this.notifyStateChanged();
          return;
        }
        this.editingRule.direction = this.ruleDirection;
        this.editingRule.priority = this.rulePriority;
        this.editingRule.action = this.ruleAction;
        this.editingRule.protocol = this.ruleProtocol;
        this.editingRule.port = this.rulePort;
        this.editingRule.description = this.ruleDescription;
        this.editingRule.enabled = this.ruleEnabled;
        this.editingRule.subjectType = peer.peerType;
        this.editingRule.subjectValue = peer.peerValue;
      }
      this.closeRuleDialog();
      this.notifyStateChanged();
      return;
    }
    try {
      const created = await this.api.post<ApiSecurityRule>(WEB_API.securityRules(this.selectedSecurityGroupId), {
        actorUserId: this.effectiveUserId,
        direction: this.ruleDirection,
        priority: this.rulePriority,
        action: this.ruleAction,
        protocol: this.ruleProtocol,
        portFrom,
        portTo,
        peerType: peer.peerType,
        peerValue: peer.peerValue,
        description: this.ruleDescription,
        enabled: this.ruleEnabled,
      });
      this.securityRules = [...this.securityRules, this.mapSecurityRule(created)];
    } catch {
      if (!this.isDemoMode) {
        this.securityRuleDialogMessage = '创建安全规则失败';
        this.notifyStateChanged();
        return;
      }
      this.securityRules = [
        ...this.securityRules,
        { direction: this.ruleDirection, priority: this.rulePriority, action: this.ruleAction, protocol: this.ruleProtocol, port: this.rulePort, description: this.ruleDescription, subjectType: peer.peerType, subjectValue: peer.peerValue, enabled: this.ruleEnabled },
      ];
    }
    this.closeRuleDialog();
    this.notifyStateChanged();
  }

  async removeSecurityRule(rule: SecurityRuleRow): Promise<void> {
    const ruleId = rule.ruleId?.trim() ?? '';
    const directionLabel = rule.direction === 'egress' ? '出方向' : '入方向';
    if (!window.confirm(`确定删除这条${directionLabel}规则吗？删除后立即生效。`)) {
      return;
    }
    this.securityRuleDialogMessage = '';
    if (!ruleId && !this.isDemoMode) {
      this.securityRuleDialogMessage = '删除安全规则失败：规则不存在，请刷新页面后重试';
      this.notifyStateChanged();
      return;
    }
    if (ruleId) {
      try {
        await this.api.delete(WEB_API.securityRule(ruleId, this.effectiveUserId));
      } catch (error) {
        if (!this.isDemoMode) {
          this.securityRuleDialogMessage = this.securityRuleFailureMessage('删除', error);
          this.notifyStateChanged();
          return;
        }
      }
    }
    this.securityRules = this.securityRules.filter((item) => ruleId ? item.ruleId !== ruleId : item !== rule);
    if (this.editingRule === rule || (ruleId && this.editingRule?.ruleId === ruleId)) {
      this.closeRuleDialog();
      this.editingRule = null;
    }
    this.notifyStateChanged();
  }

  private securityRuleFailureMessage(action: string, error: unknown): string {
    if (!(error instanceof ApiHttpError)) {
      return `${action}安全规则失败`;
    }
    const detail = ({
      invalid_argument: '来源或目标对象无效，请选择当前网络中的设备或设备分组',
      not_found: '规则不存在，请刷新页面后重试',
      forbidden: '当前用户无权修改该规则',
      unauthorized: '登录状态已失效，请重新登录',
    } as Record<string, string>)[error.code] ?? `服务端返回 HTTP ${error.status}`;
    return `${action}安全规则失败：${detail}`;
  }
}
