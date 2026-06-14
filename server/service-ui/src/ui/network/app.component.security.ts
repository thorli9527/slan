import { AppComponentDns } from './app.component.dns';
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
import { WEB_API } from '../api-paths';
import { compactUuid } from '../app.utils';


export abstract class AppComponentSecurity extends AppComponentDns {
  async removeSecurityGroup(group: SecurityGroupRow): Promise<void> {
    try {
      await this.api.delete(WEB_API.securityGroup(group.workspaceId, group.securityGroupId));
    } catch {
      // Local preview mode removes below.
    }
    this.securityGroups = this.securityGroups.filter((item) => item.securityGroupId !== group.securityGroupId);
    if (this.selectedSecurityGroupId === group.securityGroupId) {
      this.selectedSecurityGroupId = this.currentSecurityGroups[0]?.securityGroupId ?? '';
    }
  }

  openSecurityGroupDialog(): void {
    this.securityGroupName = '默认安全组';
    this.showSecurityGroupDialog = true;
  }

  closeSecurityGroupDialog(): void {
    this.showSecurityGroupDialog = false;
  }

  async saveSecurityGroupDialog(): Promise<void> {
    if (!this.securityGroupName.trim()) {
      return;
    }
    try {
      const created = await this.api.post<ApiSecurityGroup>(WEB_API.securityGroups(this.selectedWorkspaceId), {
        name: this.securityGroupName.trim(),
      });
      this.securityGroups = [...this.securityGroups, this.mapSecurityGroup(created)];
    } catch {
      this.securityGroups = [
        ...this.securityGroups,
        {
          securityGroupId: compactUuid(),
          networkId: this.selectedWorkspaceId,
          workspaceId: this.selectedWorkspaceId,
          name: this.securityGroupName.trim(),
          status: 'active',
        },
      ];
    }
    this.closeSecurityGroupDialog();
    this.notifyStateChanged();
  }

  openRuleDialog(direction: string): void {
    this.ruleDirection = direction;
    this.selectedRuleTemplate = direction === 'egress' ? '全部出站' : 'Web 服务';
    this.applyRuleTemplate(direction);
    this.editingRule = null;
    this.ruleDialogMode = 'create';
    this.showIngressRuleDialog = direction === 'ingress';
    this.showEgressRuleDialog = direction === 'egress';
  }

  openEditRuleDialog(rule: SecurityRuleRow): void {
    this.ruleDirection = rule.direction;
    this.rulePriority = rule.priority;
    this.ruleAction = rule.action;
    this.ruleProtocol = rule.protocol;
    this.rulePort = rule.port;
    this.ruleSubjectType = rule.subjectType;
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
    this.ruleSubjectType = template.subjectType;
    this.ruleSubjectValue = template.subjectValue;
    this.onRuleSubjectTypeChanged();
  }

  ruleSubjectLabel(rule: SecurityRuleRow): string {
    const prefix = this.subjectTypeLabel(rule.subjectType);
    if (rule.subjectType === 'device') {
      const device = this.devices.find((item) => item.deviceId === rule.subjectValue);
      return `${prefix}:${device ? `${this.userLabel(device.owner)} / ${device.alias || device.deviceId} / ${device.deviceId}` : rule.subjectValue}`;
    }
    if (rule.subjectType === 'workspace') {
      const workspace = rule.subjectValue === 'self' ? this.selectedWorkspace : this.workspaces.find((item) => item.workspaceId === rule.subjectValue);
      return `${prefix}:${workspace?.name ?? rule.subjectValue}`;
    }
    return `${prefix}:${rule.subjectValue}`;
  }

  subjectTypeLabel(type: RuleSubjectType): string {
    return ({ device: '设备', user: '用户', workspace: '网络', cidr: 'CIDR', domain: '域名', all: '全部' } as Record<RuleSubjectType, string>)[type];
  }

  onRuleSubjectTypeChanged(): void {
    const first = this.ruleSubjectOptions[0]?.value;
    if (first) {
      this.ruleSubjectValue = first;
      return;
    }
    this.ruleSubjectValue = this.ruleSubjectType === 'cidr' ? '0.0.0.0/0' : 'example.com';
  }

  closeRuleDialog(): void {
    this.showIngressRuleDialog = false;
    this.showEgressRuleDialog = false;
  }

  async saveRuleDialog(): Promise<void> {
    const portFrom = this.rulePort === 'all' ? 0 : Number.parseInt(this.rulePort.split(',')[0], 10) || 0;
    const portTo = this.rulePort === 'all' ? 0 : Number.parseInt(this.rulePort.split(',').at(-1) ?? this.rulePort, 10) || portFrom;
    if (this.ruleDialogMode === 'edit' && this.editingRule) {
      try {
        const updated = await this.api.patch<ApiSecurityRule>(WEB_API.securityRule(this.editingRule.ruleId ?? ''), {
          direction: this.ruleDirection,
          priority: this.rulePriority,
          action: this.ruleAction,
          protocol: this.ruleProtocol,
          portFrom,
          portTo,
          peerType: this.ruleSubjectType,
          peerValue: this.ruleSubjectValue,
          enabled: true,
        });
        Object.assign(this.editingRule, this.mapSecurityRule(updated));
      } catch {
        this.editingRule.direction = this.ruleDirection;
        this.editingRule.priority = this.rulePriority;
        this.editingRule.action = this.ruleAction;
        this.editingRule.protocol = this.ruleProtocol;
        this.editingRule.port = this.rulePort;
        this.editingRule.subjectType = this.ruleSubjectType;
        this.editingRule.subjectValue = this.ruleSubjectValue;
      }
      this.closeRuleDialog();
      this.notifyStateChanged();
      return;
    }
    try {
      const created = await this.api.post<ApiSecurityRule>(WEB_API.securityRules(this.selectedSecurityGroupId), {
        direction: this.ruleDirection,
        priority: this.rulePriority,
        action: this.ruleAction,
        protocol: this.ruleProtocol,
        portFrom,
        portTo,
        peerType: this.ruleSubjectType,
        peerValue: this.ruleSubjectValue,
        enabled: true,
      });
      this.securityRules = [...this.securityRules, this.mapSecurityRule(created)];
    } catch {
      this.securityRules = [
        ...this.securityRules,
        { direction: this.ruleDirection, priority: this.rulePriority, action: this.ruleAction, protocol: this.ruleProtocol, port: this.rulePort, subjectType: this.ruleSubjectType, subjectValue: this.ruleSubjectValue },
      ];
    }
    this.closeRuleDialog();
    this.notifyStateChanged();
  }

  async removeSecurityRule(rule: SecurityRuleRow): Promise<void> {
    try {
      await this.api.delete(WEB_API.securityRule(rule.ruleId ?? ''));
    } catch {
      // Local preview mode removes below.
    }
    this.securityRules = this.securityRules.filter((item) => item !== rule);
  }
}
