import { AppComponentDevices } from '../device/app.component.devices';
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


export abstract class AppComponentDns extends AppComponentDevices {
  openZoneDialog(): void {
    const workspace = this.selectedWorkspace;
    this.zoneDialogMessage = '';
    this.zoneName = workspace.code === 'default' ? 'default.lan' : `${workspace.code}.internal`;
    this.zoneExpose = workspace.name !== '默认网络';
    this.zoneRecordType = 'A';
    this.zoneValue = this.devices[0]?.ip ?? '10.0.0.1';
    this.editingZone = null;
    this.zoneDialogMode = 'create';
    this.showZoneDialog = true;
  }

  openEditZoneDialog(zone: DNSZoneRow): void {
    this.zoneDialogMessage = '';
    this.zoneName = zone.zone;
    this.zoneExpose = zone.expose;
    this.zoneRecordType = zone.recordType;
    this.zoneValue = zone.value;
    this.editingZone = zone;
    this.zoneDialogMode = 'edit';
    this.showZoneDialog = true;
  }

  openZoneTagDialog(zone: DNSZoneRow): void {
    if (this.showZoneTagDialog && this.editingZone === zone) {
      this.closeZoneTagDialog();
      return;
    }
    this.editingZone = zone;
    this.zoneDialogMessage = '';
    this.zoneNameValue = zone.zone;
    this.zoneExposeValue = zone.expose;
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
          exposeGlobal: this.zoneExposeValue,
        });
        Object.assign(this.editingZone, this.mapDNSZone(updated));
      } catch {
        if (!this.isDemoMode) {
          this.zoneDialogMessage = '更新 DNS Zone 失败';
          this.notifyStateChanged();
          return;
        }
        this.editingZone.zone = zoneName;
        this.editingZone.expose = this.zoneExposeValue;
      }
    } else {
      this.editingZone.zone = zoneName;
      this.editingZone.expose = this.zoneExposeValue;
    }
    this.closeZoneTagDialog();
  }

  closeZoneDialog(): void {
    this.showZoneDialog = false;
    this.zoneDialogMessage = '';
  }

  async saveZoneDialog(): Promise<void> {
    this.zoneDialogMessage = '';
    const workspace = this.selectedWorkspace;
    const zone = this.normalizePrivateZone(this.zoneName);
    if (this.zoneDialogMode === 'edit' && this.editingZone) {
      const zoneId = this.resourceId(this.editingZone.zoneId);
      if (zoneId) {
        try {
          const updated = await this.api.patch<ApiDNSZone>(WEB_API.dnsZone(workspace.workspaceId, zoneId, this.effectiveUserId), {
            actorUserId: this.effectiveUserId,
            zoneName: zone,
            exposeGlobal: this.zoneExpose,
          });
          Object.assign(this.editingZone, this.mapDNSZone(updated));
        } catch {
          if (!this.isDemoMode) {
            this.zoneDialogMessage = '更新 DNS Zone 失败';
            this.notifyStateChanged();
            return;
          }
          this.editingZone.zone = zone;
          this.editingZone.expose = this.zoneExpose;
          this.editingZone.recordType = this.zoneRecordType;
          this.editingZone.value = this.zoneValue;
        }
      } else {
        this.editingZone.zone = zone;
        this.editingZone.expose = this.zoneExpose;
        this.editingZone.recordType = this.zoneRecordType;
        this.editingZone.value = this.zoneValue;
      }
      this.closeZoneDialog();
      this.notifyStateChanged();
      return;
    }
      try {
        const created = await this.api.post<ApiDNSZone>(WEB_API.dnsZones(workspace.workspaceId), {
          actorUserId: this.effectiveUserId,
          zoneName: zone,
          exposeGlobal: this.zoneExpose,
        });
        this.dnsZones = [...this.dnsZones, this.mapDNSZone(created)];
      } catch {
        if (!this.isDemoMode) {
          this.zoneDialogMessage = '创建 DNS Zone 失败';
          this.notifyStateChanged();
          return;
        }
        this.dnsZones = [
          ...this.dnsZones,
          { zoneId: this.localResourceId('zone'), networkId: workspace.networkId, workspaceId: workspace.workspaceId, zone, recordType: this.zoneRecordType, value: this.zoneValue, expose: this.zoneExpose, status: 'active' },
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
    this.recordDialogMessage = '';
    this.recordName = this.domainName;
    this.recordType = 'A';
    this.recordTargetType = 'device';
    this.recordDeviceId = this.workspaceDevices[0]?.deviceId ?? this.devices[0]?.deviceId ?? '';
    this.recordTargetIp = '10.0.0.10';
    this.recordCname = `${this.selectedWorkspace.code}.internal`;
    this.recordPort = '443';
    this.recordValue = this.buildRecordValue();
    this.editingRecord = null;
    this.recordDialogMode = 'create';
    this.showRecordDialog = true;
  }

  openEditRecordDialog(record: DNSRow): void {
    this.recordDialogMessage = '';
    this.recordName = record.name;
    this.recordType = record.recordType;
    this.recordTargetType = record.targetType ?? (record.deviceId ? 'device' : 'ip');
    this.recordDeviceId = record.deviceId || this.workspaceDevices[0]?.deviceId || '';
    this.recordTargetIp = this.recordTargetType === 'ip' ? record.value : '';
    this.recordCname = this.recordTargetType === 'cname' ? record.value : '';
    this.recordPort = record.port || '443';
    this.recordValue = this.buildRecordValue();
    this.editingRecord = record;
    this.recordDialogMode = 'edit';
    this.showRecordDialog = true;
  }

  buildRecordValue(): string {
    if (this.recordTargetType === 'ip') {
      return this.recordTargetIp.trim();
    }
    if (this.recordTargetType === 'cname') {
      return this.recordCname.trim().toLowerCase();
    }
    const device = this.devices.find((item) => item.deviceId === this.recordDeviceId);
    if (!device) {
      return `- / - / ${this.recordPort}`;
    }
    return `${this.userLabel(device.owner)} / ${device.alias || device.deviceId} / ${this.recordPort}`;
  }

  publicMappingDeviceLabel(mapping: PublicMappingRow): string {
    const device = this.devices.find((item) => item.deviceId === mapping.deviceId);
    if (!device) {
      return mapping.deviceId || '-';
    }
    return `${this.userLabel(device.owner)} / ${device.alias || device.deviceId}`;
  }

  syncRecordValue(): void {
    this.recordValue = this.buildRecordValue();
  }

  syncRecordType(): void {
    if (this.recordType === 'CNAME') {
      this.recordTargetType = 'cname';
    } else if (this.recordTargetType === 'cname') {
      this.recordTargetType = 'device';
    }
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
    const zone = zoneRow?.zone ?? `${workspace.code}.internal`;
    const fqdn = `${name}.${zone}`;
    this.recordValue = this.buildRecordValue();
    const targetDeviceId = this.recordTargetType === 'device' ? this.recordDeviceId : '';
    const targetIp = this.recordTargetType === 'ip' ? this.recordTargetIp.trim() : '';
    const cname = this.recordTargetType === 'cname' ? this.recordCname.trim().toLowerCase() : '';
    if (this.recordDialogMode === 'edit' && this.editingRecord) {
      const recordId = this.resourceId(this.editingRecord.recordId);
      if (recordId) {
        try {
          const updated = await this.api.patch<ApiDNSRecord>(WEB_API.dnsRecord(workspace.workspaceId, recordId, this.effectiveUserId), {
            actorUserId: this.effectiveUserId,
            name,
            recordType: this.recordType,
            targetDeviceId,
            targetIp,
            cname,
            port: this.recordTargetType === 'device' ? this.recordPort : '',
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
          this.editingRecord.port = this.recordTargetType === 'device' ? this.recordPort : '';
          this.editingRecord.ttl = 60;
          this.editingRecord.targetType = this.recordTargetType;
        }
      } else {
        this.editingRecord.name = name;
        this.editingRecord.fqdn = fqdn;
        this.editingRecord.recordType = this.recordType;
        this.editingRecord.value = this.recordValue;
        this.editingRecord.deviceId = targetDeviceId;
        this.editingRecord.port = this.recordTargetType === 'device' ? this.recordPort : '';
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
          targetIp,
          cname,
          port: this.recordTargetType === 'device' ? this.recordPort : '',
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
          { recordId: this.localResourceId('record'), zoneId, networkId: workspace.networkId, workspaceId: workspace.workspaceId, name, fqdn, recordType: this.recordType, value: this.recordValue, deviceId: targetDeviceId, port: this.recordTargetType === 'device' ? this.recordPort : '', ttl: 60, targetType: this.recordTargetType, expose: false },
        ];
      }
    } else {
      this.dnsRecords = [
        ...this.dnsRecords,
        { recordId: this.localResourceId('record'), networkId: workspace.networkId, workspaceId: workspace.workspaceId, name, fqdn, recordType: this.recordType, value: this.recordValue, deviceId: targetDeviceId, port: this.recordTargetType === 'device' ? this.recordPort : '', ttl: 60, targetType: this.recordTargetType, expose: false },
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

  openPublicMappingDialog(): void {
    const device = this.currentUserDevices[0];
    this.publicMappingDialogMessage = '';
    this.publicAlias = 'api';
    this.publicTargetType = 'device';
    this.publicSourceRecord = device?.deviceId ?? '';
    this.publicInternalIp = '10.0.0.10';
    this.publicProtocol = 'HTTP';
    this.publicInternalPort = '8443';
    this.publicExternalPort = '443';
    this.publicAccessMode = 'public';
    this.publicTlsMode = 'auto';
    this.editingPublicMapping = null;
    this.publicMappingDialogMode = 'create';
    this.showPublicMappingDialog = true;
  }

  openEditPublicMappingDialog(mapping: PublicMappingRow): void {
    this.publicMappingDialogMessage = '';
    const targetType = mapping.targetType ?? (mapping.deviceId ? 'device' : mapping.internalIp ? 'ip' : 'device');
    const record = targetType === 'record'
      ? this.currentDNSRecords.find((item) => item.fqdn === mapping.sourceRecord || item.name === mapping.sourceRecord)
      : null;
    this.publicAlias = mapping.alias;
    this.publicTargetType = targetType;
    this.publicSourceRecord = targetType === 'device'
      ? mapping.deviceId
      : targetType === 'record'
        ? (record?.recordId ?? '')
        : mapping.sourceRecord;
    this.publicInternalIp = mapping.internalIp || '';
    this.publicProtocol = mapping.protocol;
    this.publicInternalPort = mapping.internalPort || mapping.port;
    this.publicExternalPort = mapping.externalPort;
    this.publicAccessMode = mapping.accessMode;
    this.publicTlsMode = mapping.tlsMode;
    this.editingPublicMapping = mapping;
    this.publicMappingDialogMode = 'edit';
    this.showPublicMappingDialog = true;
  }

  closePublicMappingDialog(): void {
    this.showPublicMappingDialog = false;
    this.publicMappingDialogMessage = '';
  }

  async savePublicMappingDialog(): Promise<void> {
    this.publicMappingDialogMessage = '';
    const device = this.currentUserDevices.find((item) => item.deviceId === this.publicSourceRecord) ?? this.currentUserDevices[0];
    const record = this.currentDNSRecords.find((item) => (item.recordId ?? '') === this.publicSourceRecord);
    const recordDeviceID = record?.targetType === 'device' ? record.deviceId : '';
    const recordInternalIP = record?.targetType === 'ip' ? record.value.trim() : '';
    const deviceID = this.publicTargetType === 'device' ? device?.deviceId ?? '' : this.publicTargetType === 'record' ? recordDeviceID : '';
    const internalIP = this.publicTargetType === 'ip' ? this.publicInternalIp.trim() : this.publicTargetType === 'record' ? recordInternalIP : '';
    const sourceRecord = this.publicTargetType === 'device'
      ? device?.deviceId || ''
      : this.publicTargetType === 'record'
        ? (record?.fqdn || record?.name || '')
        : internalIP;
    const alias = slug(this.publicAlias);
    const publicDomain = `${alias}.${this.selectedWorkspace.code}.${this.userSlug}.pub.staticlss.com`;
    const internalPort = this.publicInternalPort.trim() || this.publicExternalPort.trim();
    if (this.publicMappingDialogMode === 'edit' && this.editingPublicMapping) {
      const mappingId = this.resourceId(this.editingPublicMapping.mappingId);
      if (mappingId) {
        try {
          const updated = await this.api.patch<ApiPublicMapping>(WEB_API.publicMapping(this.selectedWorkspaceId, mappingId, this.effectiveUserId), {
            actorUserId: this.effectiveUserId,
            alias,
            publicDomain,
            sourceRecord,
            deviceId: deviceID,
            internalIp: internalIP,
            protocol: this.publicProtocol,
            internalPort: Number.parseInt(internalPort, 10) || 0,
            port: internalPort,
            externalPort: this.publicExternalPort,
            accessMode: this.publicAccessMode,
            tlsMode: this.publicTlsMode,
            status: this.editingPublicMapping.status,
          });
          Object.assign(this.editingPublicMapping, {
            ...this.mapPublicMapping(updated),
            accessMode: this.publicAccessMode,
            tlsMode: this.publicTlsMode,
          });
        } catch {
          if (!this.isDemoMode) {
            this.publicMappingDialogMessage = '更新公网映射失败';
            this.notifyStateChanged();
            return;
          }
          this.editingPublicMapping.alias = alias;
          this.editingPublicMapping.publicDomain = publicDomain;
          this.editingPublicMapping.sourceRecord = sourceRecord;
          this.editingPublicMapping.deviceId = deviceID;
          this.editingPublicMapping.internalIp = internalIP;
          this.editingPublicMapping.targetType = this.publicTargetType;
          this.editingPublicMapping.protocol = this.publicProtocol;
          this.editingPublicMapping.internalPort = internalPort;
          this.editingPublicMapping.port = internalPort;
          this.editingPublicMapping.externalPort = this.publicExternalPort;
          this.editingPublicMapping.accessMode = this.publicAccessMode;
          this.editingPublicMapping.tlsMode = this.publicTlsMode;
        }
      } else {
        this.editingPublicMapping.alias = alias;
        this.editingPublicMapping.publicDomain = publicDomain;
        this.editingPublicMapping.sourceRecord = sourceRecord;
        this.editingPublicMapping.deviceId = deviceID;
        this.editingPublicMapping.internalIp = internalIP;
        this.editingPublicMapping.targetType = this.publicTargetType;
        this.editingPublicMapping.protocol = this.publicProtocol;
        this.editingPublicMapping.internalPort = internalPort;
        this.editingPublicMapping.port = internalPort;
        this.editingPublicMapping.externalPort = this.publicExternalPort;
        this.editingPublicMapping.accessMode = this.publicAccessMode;
        this.editingPublicMapping.tlsMode = this.publicTlsMode;
      }
      this.closePublicMappingDialog();
      return;
    }
    try {
      const created = await this.api.post<ApiPublicMapping>(WEB_API.publicMappings(this.selectedWorkspaceId), {
        actorUserId: this.effectiveUserId,
        alias,
        publicDomain,
        sourceRecord,
        deviceId: deviceID,
        internalIp: internalIP,
        protocol: this.publicProtocol,
        internalPort: Number.parseInt(internalPort, 10) || 0,
        port: internalPort,
        externalPort: this.publicExternalPort,
        accessMode: this.publicAccessMode,
        tlsMode: this.publicTlsMode,
        status: 'enabled',
      });
      this.publicMappings = [...this.publicMappings, { ...this.mapPublicMapping(created), accessMode: this.publicAccessMode, tlsMode: this.publicTlsMode }];
    } catch {
      if (!this.isDemoMode) {
        this.publicMappingDialogMessage = '创建公网映射失败';
        this.notifyStateChanged();
        return;
      }
      this.publicMappings = [
        ...this.publicMappings,
        { mappingId: this.localResourceId('mapping'), networkId: this.selectedWorkspaceId, workspaceId: this.selectedWorkspaceId, alias, publicDomain, sourceRecord, deviceId: deviceID, internalIp: internalIP, targetType: this.publicTargetType, protocol: this.publicProtocol, internalPort, port: internalPort, externalPort: this.publicExternalPort, accessMode: this.publicAccessMode, tlsMode: this.publicTlsMode, status: 'enabled' },
      ];
    }
    this.closePublicMappingDialog();
    this.notifyStateChanged();
  }

  async removePublicMapping(mapping: PublicMappingRow): Promise<void> {
    const mappingId = this.resourceId(mapping.mappingId);
    if (mappingId) {
      try {
        await this.api.delete(WEB_API.publicMapping(mapping.workspaceId, mappingId, this.effectiveUserId));
      } catch {
        if (!this.isDemoMode) {
          this.publicMappingDialogMessage = '删除公网映射失败';
          this.notifyStateChanged();
          return;
        }
      }
    }
    this.publicMappings = this.publicMappings.filter((item) => item !== mapping);
    this.notifyStateChanged();
  }

  protected resourceId(value: string | undefined): string | undefined {
    const id = value?.trim() ?? '';
    return id || undefined;
  }

  protected localResourceId(_prefix: string): string {
    return compactUuid();
  }
}
