import { AppComponentDevices } from '../device/app.component.devices';
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


export abstract class AppComponentDns extends AppComponentDevices {
  openZoneDialog(): void {
    this.closeInlinePopovers();
    const workspace = this.selectedWorkspace;
    this.zoneDialogMessage = '';
    this.zoneName = `${slug(workspace.name)}.internal`;
    this.zoneRecordType = 'A';
    this.zoneValue = this.devices[0]?.ip ?? '10.0.0.1';
    this.editingZone = null;
    this.zoneDialogMode = 'create';
    this.showZoneDialog = true;
  }

  openEditZoneDialog(zone: DNSZoneRow): void {
    this.closeInlinePopovers();
    this.zoneDialogMessage = '';
    this.zoneName = zone.zone;
    this.zoneRecordType = zone.recordType;
    this.zoneValue = zone.value;
    this.editingZone = zone;
    this.zoneDialogMode = 'edit';
    this.showZoneDialog = true;
  }

  openZoneTagDialog(zone: DNSZoneRow): void {
    this.closeInlinePopovers();
    if (this.showZoneTagDialog && this.editingZone === zone) {
      this.closeZoneTagDialog();
      return;
    }
    this.editingZone = zone;
    this.zoneDialogMessage = '';
    this.zoneNameValue = zone.zone;
    this.showZoneTagDialog = true;
  }

  closeZoneTagDialog(): void {
    this.showZoneTagDialog = false;
    this.editingZone = null;
    this.zoneDialogMessage = '';
  }

  async saveZoneTagDialog(): Promise<void> {
    if (!this.editingZone || !this.zoneNameValue.trim()) {
      return;
    }
    this.zoneDialogMessage = '';
    const zoneName = this.normalizePrivateZone(this.zoneNameValue);
    const zoneId = this.resourceId(this.editingZone.zoneId);
    if (zoneId) {
      try {
        const updated = await this.api.patch<ApiDNSZone>(WEB_API.dnsZone(this.editingZone.workspaceId, zoneId, this.effectiveUserId), {
          actorUserId: this.effectiveUserId,
          zoneName,
        });
        Object.assign(this.editingZone, this.mapDNSZone(updated));
      } catch (error) {
        if (!this.isDemoMode) {
          this.zoneDialogMessage = this.zoneRequestError('更新', error);
          this.notifyStateChanged();
          return;
        }
        this.editingZone.zone = zoneName;
      }
    } else {
      this.editingZone.zone = zoneName;
    }
    this.closeZoneTagDialog();
  }

  closeZoneDialog(): void {
    this.showZoneDialog = false;
    this.zoneDialogMessage = '';
  }

  async saveZoneDialog(): Promise<void> {
    this.zoneDialogMessage = '';
    const networkId = this.selectedWorkspaceId.trim();
    if (!networkId) {
      this.zoneDialogMessage = '创建 DNS Zone 失败：当前网络无效';
      this.notifyStateChanged();
      return;
    }
    const zone = this.normalizePrivateZone(this.zoneName);
    if (this.zoneDialogMode === 'edit' && this.editingZone) {
      const zoneId = this.resourceId(this.editingZone.zoneId);
      if (zoneId) {
        try {
          const updated = await this.api.patch<ApiDNSZone>(WEB_API.dnsZone(networkId, zoneId, this.effectiveUserId), {
            actorUserId: this.effectiveUserId,
            zoneName: zone,
          });
          Object.assign(this.editingZone, this.mapDNSZone(updated));
        } catch (error) {
          if (!this.isDemoMode) {
            this.zoneDialogMessage = this.zoneRequestError('更新', error);
            this.notifyStateChanged();
            return;
          }
          this.editingZone.zone = zone;
          this.editingZone.recordType = this.zoneRecordType;
          this.editingZone.value = this.zoneValue;
        }
      } else {
        this.editingZone.zone = zone;
        this.editingZone.recordType = this.zoneRecordType;
        this.editingZone.value = this.zoneValue;
      }
      this.closeZoneDialog();
      this.notifyStateChanged();
      return;
    }
    try {
      const created = await this.api.post<ApiDNSZone>(WEB_API.dnsZones(networkId), {
        actorUserId: this.effectiveUserId,
        zoneName: zone,
      });
      this.dnsZones = [...this.dnsZones, this.mapDNSZone(created)];
    } catch (error) {
      if (!this.isDemoMode) {
        this.zoneDialogMessage = this.zoneRequestError('创建', error);
        this.notifyStateChanged();
        return;
      }
      this.dnsZones = [
        ...this.dnsZones,
        { zoneId: this.localResourceId('zone'), networkId, workspaceId: networkId, zone, recordType: this.zoneRecordType, value: this.zoneValue, status: 'active' },
      ];
    }
    this.closeZoneDialog();
    this.notifyStateChanged();
  }

  async removeZone(zone: DNSZoneRow): Promise<void> {
    const zoneId = this.resourceId(zone.zoneId);
    if (zoneId) {
      try {
        await this.api.delete(WEB_API.dnsZone(zone.workspaceId, zoneId, this.effectiveUserId));
      } catch {
        if (!this.isDemoMode) {
          this.zoneDialogMessage = '删除 DNS Zone 失败';
          this.notifyStateChanged();
          return;
        }
      }
    }
    this.dnsZones = this.dnsZones.filter((item) => item !== zone);
  }

  openRecordDialog(): void {
    this.closeInlinePopovers();
    this.recordDialogMessage = '';
    this.recordName = this.domainName;
    this.recordType = 'A';
    this.recordTargetType = 'device';
    this.recordDeviceId = this.workspaceDevices[0]?.deviceId ?? '';
    this.recordCname = `${slug(this.selectedWorkspace.name)}.internal`;
    this.recordValue = this.buildRecordValue();
    this.editingRecord = null;
    this.recordDialogMode = 'create';
    this.showRecordDialog = true;
  }

  openEditRecordDialog(record: DNSRow): void {
    this.closeInlinePopovers();
    this.recordDialogMessage = '';
    this.recordName = record.name;
    this.recordType = record.recordType;
    this.recordTargetType = record.recordType === 'CNAME' ? 'cname' : 'device';
    this.recordDeviceId = record.deviceId || this.workspaceDevices[0]?.deviceId || '';
    this.recordCname = this.recordTargetType === 'cname' ? record.value : '';
    this.recordValue = this.buildRecordValue();
    this.editingRecord = record;
    this.recordDialogMode = 'edit';
    this.showRecordDialog = true;
  }

  buildRecordValue(): string {
    if (this.recordTargetType === 'cname') {
      return this.recordCname.trim().toLowerCase();
    }
    const device = this.devices.find((item) => item.deviceId === this.recordDeviceId);
    if (!device) {
      return '- / -';
    }
    return `${this.userLabel(device.owner)} / ${device.alias || device.deviceId}`;
  }

  syncRecordValue(): void {
    this.recordValue = this.buildRecordValue();
  }

  syncRecordType(): void {
    this.recordTargetType = this.recordType === 'CNAME' ? 'cname' : 'device';
    this.syncRecordValue();
  }

  closeRecordDialog(): void {
    this.showRecordDialog = false;
    this.recordDialogMessage = '';
  }

  async saveRecordDialog(): Promise<void> {
    this.recordDialogMessage = '';
    const workspace = this.selectedWorkspace;
    const name = slug(this.recordName);
    const zoneRow = this.currentDNSZones.find((item) => item.zoneId === this.selectedZoneId) ?? this.currentDNSZones[0];
    const zone = zoneRow?.zone ?? `${slug(workspace.name)}.internal`;
    const fqdn = `${name}.${zone}`;
    this.recordValue = this.buildRecordValue();
    const targetDeviceId = this.recordTargetType === 'device' ? this.recordDeviceId : '';
    const cname = this.recordTargetType === 'cname' ? this.recordCname.trim().toLowerCase() : '';
    if ((this.recordTargetType === 'device' && !targetDeviceId) || (this.recordTargetType === 'cname' && !cname)) {
      this.recordDialogMessage = this.recordTargetType === 'device' ? '请选择当前网络中的目标设备' : '请输入目标 CNAME';
      this.notifyStateChanged();
      return;
    }
    if (this.recordDialogMode === 'edit' && this.editingRecord) {
      const recordId = this.resourceId(this.editingRecord.recordId);
      if (recordId) {
        try {
          const updated = await this.api.patch<ApiDNSRecord>(WEB_API.dnsRecord(workspace.workspaceId, recordId, this.effectiveUserId), {
            actorUserId: this.effectiveUserId,
            name,
            recordType: this.recordType,
            targetDeviceId,
            cname,
            ttl: 60,
          });
          Object.assign(this.editingRecord, this.mapDNSRecord(updated));
        } catch {
          if (!this.isDemoMode) {
            this.recordDialogMessage = '更新 DNS 记录失败';
            this.notifyStateChanged();
            return;
          }
          this.editingRecord.name = name;
          this.editingRecord.fqdn = fqdn;
          this.editingRecord.recordType = this.recordType;
          this.editingRecord.value = this.recordValue;
          this.editingRecord.deviceId = targetDeviceId;
          this.editingRecord.ttl = 60;
          this.editingRecord.targetType = this.recordTargetType;
        }
      } else {
        this.editingRecord.name = name;
        this.editingRecord.fqdn = fqdn;
        this.editingRecord.recordType = this.recordType;
        this.editingRecord.value = this.recordValue;
        this.editingRecord.deviceId = targetDeviceId;
        this.editingRecord.ttl = 60;
        this.editingRecord.targetType = this.recordTargetType;
      }
      this.closeRecordDialog();
      this.notifyStateChanged();
      return;
    }
    const zoneId = this.resourceId(zoneRow?.zoneId);
    if (zoneId) {
      try {
        const created = await this.api.post<ApiDNSRecord>(WEB_API.dnsRecords(workspace.workspaceId), {
          actorUserId: this.effectiveUserId,
          zoneId,
          name,
          recordType: this.recordType,
          targetDeviceId,
          cname,
          ttl: 60,
        });
        this.dnsRecords = [...this.dnsRecords, this.mapDNSRecord(created)];
      } catch {
        if (!this.isDemoMode) {
          this.recordDialogMessage = '创建 DNS 记录失败';
          this.notifyStateChanged();
          return;
        }
        this.dnsRecords = [
          ...this.dnsRecords,
          { recordId: this.localResourceId('record'), zoneId, networkId: workspace.networkId, workspaceId: workspace.workspaceId, name, fqdn, recordType: this.recordType, value: this.recordValue, deviceId: targetDeviceId, ttl: 60, targetType: this.recordTargetType },
        ];
      }
    } else {
      this.dnsRecords = [
        ...this.dnsRecords,
        { recordId: this.localResourceId('record'), networkId: workspace.networkId, workspaceId: workspace.workspaceId, name, fqdn, recordType: this.recordType, value: this.recordValue, deviceId: targetDeviceId, ttl: 60, targetType: this.recordTargetType },
      ];
    }
    this.closeRecordDialog();
    this.notifyStateChanged();
  }

  async removeDomainRecord(record: DNSRow): Promise<void> {
    const recordId = this.resourceId(record.recordId);
    if (recordId) {
      try {
        await this.api.delete(WEB_API.dnsRecord(record.workspaceId, recordId, this.effectiveUserId));
      } catch {
        if (!this.isDemoMode) {
          this.recordDialogMessage = '删除 DNS 记录失败';
          this.notifyStateChanged();
          return;
        }
      }
    }
    this.dnsRecords = this.dnsRecords.filter((item) => item !== record);
    this.notifyStateChanged();
  }

  private normalizePrivateZone(value: string): string {
    return value
      .trim()
      .toLowerCase()
      .replace(/^https?:\/\//, '')
      .replace(/\/.*$/, '')
      .replace(/[^a-z0-9.-]+/g, '-')
      .replace(/^-+|-+$/g, '') || 'internal.lan';
  }

  private zoneRequestError(action: string, error: unknown): string {
    const detail = error instanceof Error ? error.message.trim() : String(error).trim();
    return `${action} DNS Zone 失败${detail ? `：${detail}` : ''}`;
  }

  protected resourceId(value: string | undefined): string | undefined {
    const id = value?.trim() ?? '';
    return id || undefined;
  }

  protected localResourceId(_prefix: string): string {
    return compactUuid();
  }
}
