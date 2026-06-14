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
import { slug } from '../app.utils';
import { WEB_API } from '../api-paths';


export abstract class AppComponentDns extends AppComponentDevices {
  openZoneDialog(): void {
    const workspace = this.selectedWorkspace;
    this.zoneName = workspace.code === 'default' ? 'default.lan' : `${workspace.code}.internal`;
    this.zoneRecordType = 'A';
    this.zoneValue = this.devices[0]?.ip ?? '10.0.0.1';
    this.editingZone = null;
    this.zoneDialogMode = 'create';
    this.showZoneDialog = true;
  }

  openEditZoneDialog(zone: DNSZoneRow): void {
    this.zoneName = zone.zone;
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
    this.zoneNameValue = zone.zone;
    this.showZoneTagDialog = true;
  }

  closeZoneTagDialog(): void {
    this.showZoneTagDialog = false;
    this.editingZone = null;
  }

  async saveZoneTagDialog(): Promise<void> {
    if (!this.editingZone || !this.zoneNameValue.trim()) {
      return;
    }
    const zoneName = this.normalizePrivateZone(this.zoneNameValue);
    const zoneId = this.resourceId(this.editingZone.zoneId);
    if (zoneId) {
      try {
        const updated = await this.api.patch<ApiDNSZone>(WEB_API.dnsZone(this.editingZone.workspaceId, zoneId), {
          zoneName,
          exposeGlobal: this.editingZone.expose,
        });
        Object.assign(this.editingZone, this.mapDNSZone(updated));
      } catch {
        this.editingZone.zone = zoneName;
      }
    } else {
      this.editingZone.zone = zoneName;
    }
    this.closeZoneTagDialog();
  }

  closeZoneDialog(): void {
    this.showZoneDialog = false;
  }

  async saveZoneDialog(): Promise<void> {
    const workspace = this.selectedWorkspace;
    const zone = this.normalizePrivateZone(this.zoneName);
    if (this.zoneDialogMode === 'edit' && this.editingZone) {
      const zoneId = this.resourceId(this.editingZone.zoneId);
      if (zoneId) {
        try {
          const updated = await this.api.patch<ApiDNSZone>(WEB_API.dnsZone(workspace.workspaceId, zoneId), {
            zoneName: zone,
            exposeGlobal: this.editingZone.expose,
          });
          Object.assign(this.editingZone, this.mapDNSZone(updated));
        } catch {
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
      const created = await this.api.post<ApiDNSZone>(WEB_API.dnsZones(workspace.workspaceId), {
        zoneName: zone,
        exposeGlobal: workspace.name !== '默认网络',
      });
      this.dnsZones = [...this.dnsZones, this.mapDNSZone(created)];
    } catch {
      this.dnsZones = [
        ...this.dnsZones,
        { zoneId: this.localResourceId('zone'), networkId: workspace.networkId, workspaceId: workspace.workspaceId, zone, recordType: this.zoneRecordType, value: this.zoneValue, expose: workspace.name !== '默认网络', status: 'active' },
      ];
    }
    this.closeZoneDialog();
    this.notifyStateChanged();
  }

  async removeZone(zone: DNSZoneRow): Promise<void> {
    const zoneId = this.resourceId(zone.zoneId);
    if (zoneId) {
      try {
        await this.api.delete(WEB_API.dnsZone(zone.workspaceId, zoneId));
      } catch {
        // Local preview mode removes below.
      }
    }
    this.dnsZones = this.dnsZones.filter((item) => item !== zone);
  }

  openRecordDialog(): void {
    this.recordName = this.domainName;
    this.recordType = 'A';
    this.recordDeviceId = this.workspaceDevices[0]?.deviceId ?? this.devices[0]?.deviceId ?? '';
    this.recordPort = '443';
    this.recordValue = this.buildRecordValue();
    this.editingRecord = null;
    this.recordDialogMode = 'create';
    this.showRecordDialog = true;
  }

  openEditRecordDialog(record: DNSRow): void {
    this.recordName = record.name;
    this.recordType = record.recordType;
    this.recordDeviceId = record.deviceId || this.workspaceDevices[0]?.deviceId || '';
    this.recordPort = record.port || '443';
    this.recordValue = this.buildRecordValue();
    this.editingRecord = record;
    this.recordDialogMode = 'edit';
    this.showRecordDialog = true;
  }

  buildRecordValue(): string {
    const device = this.devices.find((item) => item.deviceId === this.recordDeviceId);
    if (!device) {
      return `- / - / ${this.recordPort}`;
    }
    return `${this.userLabel(device.owner)} / ${device.alias || device.deviceId} / ${this.recordPort}`;
  }

  dnsRecordValue(record: DNSRow): string {
    const device = this.devices.find((item) => item.deviceId === record.deviceId);
    if (!device) {
      return this.displayUserText(record.value || '-');
    }
    return `${this.userLabel(device.owner)} / ${device.alias || device.deviceId} / ${record.port || '-'}`;
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

  closeRecordDialog(): void {
    this.showRecordDialog = false;
  }

  async saveRecordDialog(): Promise<void> {
    const workspace = this.selectedWorkspace;
    const name = slug(this.recordName);
    const zoneRow = this.currentDNSZones.find((item) => item.zoneId === this.selectedZoneId) ?? this.currentDNSZones[0];
    const zone = zoneRow?.zone ?? `${workspace.code}.internal`;
    const fqdn = `${name}.${zone}`;
    this.recordValue = this.buildRecordValue();
    if (this.recordDialogMode === 'edit' && this.editingRecord) {
      const recordId = this.resourceId(this.editingRecord.recordId);
      if (recordId) {
        try {
          const updated = await this.api.patch<ApiDNSRecord>(WEB_API.dnsRecord(workspace.workspaceId, recordId), {
            name,
            recordType: this.recordType,
            targetDeviceId: this.recordDeviceId,
            port: this.recordPort,
            ttl: 60,
          });
          Object.assign(this.editingRecord, this.mapDNSRecord(updated));
        } catch {
          this.editingRecord.name = name;
          this.editingRecord.fqdn = fqdn;
          this.editingRecord.recordType = this.recordType;
          this.editingRecord.value = this.recordValue;
          this.editingRecord.deviceId = this.recordDeviceId;
          this.editingRecord.port = this.recordPort;
        }
      } else {
        this.editingRecord.name = name;
        this.editingRecord.fqdn = fqdn;
        this.editingRecord.recordType = this.recordType;
        this.editingRecord.value = this.recordValue;
        this.editingRecord.deviceId = this.recordDeviceId;
        this.editingRecord.port = this.recordPort;
      }
      this.closeRecordDialog();
      this.notifyStateChanged();
      return;
    }
    const zoneId = this.resourceId(zoneRow?.zoneId);
    if (zoneId) {
      try {
        const created = await this.api.post<ApiDNSRecord>(WEB_API.dnsRecords(workspace.workspaceId), {
          zoneId,
          name,
          recordType: this.recordType,
          targetDeviceId: this.recordDeviceId,
          port: this.recordPort,
          ttl: 60,
        });
        this.dnsRecords = [...this.dnsRecords, this.mapDNSRecord(created)];
      } catch {
        this.dnsRecords = [
          ...this.dnsRecords,
          { recordId: this.localResourceId('record'), zoneId, networkId: workspace.networkId, workspaceId: workspace.workspaceId, name, fqdn, recordType: this.recordType, value: this.recordValue, deviceId: this.recordDeviceId, port: this.recordPort, expose: false },
        ];
      }
    } else {
      this.dnsRecords = [
        ...this.dnsRecords,
        { recordId: this.localResourceId('record'), networkId: workspace.networkId, workspaceId: workspace.workspaceId, name, fqdn, recordType: this.recordType, value: this.recordValue, deviceId: this.recordDeviceId, port: this.recordPort, expose: false },
      ];
    }
    this.closeRecordDialog();
    this.notifyStateChanged();
  }

  async removeDomainRecord(record: DNSRow): Promise<void> {
    const recordId = this.resourceId(record.recordId);
    if (recordId) {
      try {
        await this.api.delete(WEB_API.dnsRecord(record.workspaceId, recordId));
      } catch {
        // Local preview mode removes below.
      }
    }
    this.dnsRecords = this.dnsRecords.filter((item) => item !== record);
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
    this.publicAlias = 'api';
    this.publicSourceRecord = device?.deviceId ?? '';
    this.publicProtocol = 'HTTP';
    this.publicExternalPort = '443';
    this.publicAccessMode = 'public';
    this.publicTlsMode = 'auto';
    this.editingPublicMapping = null;
    this.publicMappingDialogMode = 'create';
    this.showPublicMappingDialog = true;
  }

  openEditPublicMappingDialog(mapping: PublicMappingRow): void {
    this.publicAlias = mapping.alias;
    this.publicSourceRecord = mapping.deviceId || mapping.sourceRecord;
    this.publicProtocol = mapping.protocol;
    this.publicExternalPort = mapping.externalPort;
    this.publicAccessMode = mapping.accessMode;
    this.publicTlsMode = mapping.tlsMode;
    this.editingPublicMapping = mapping;
    this.publicMappingDialogMode = 'edit';
    this.showPublicMappingDialog = true;
  }

  closePublicMappingDialog(): void {
    this.showPublicMappingDialog = false;
  }

  async savePublicMappingDialog(): Promise<void> {
    const device = this.currentUserDevices.find((item) => item.deviceId === this.publicSourceRecord) ?? this.currentUserDevices[0];
    const alias = slug(this.publicAlias);
    const publicDomain = `${alias}.${this.selectedWorkspace.code}.${this.userSlug}.pub.staticlss.com`;
    if (this.publicMappingDialogMode === 'edit' && this.editingPublicMapping) {
      const mappingId = this.resourceId(this.editingPublicMapping.mappingId);
      if (mappingId) {
        try {
          const updated = await this.api.patch<ApiPublicMapping>(WEB_API.publicMapping(this.selectedWorkspaceId, mappingId), {
            alias,
            publicDomain,
            sourceRecord: device?.alias || device?.deviceId || '',
            deviceId: device?.deviceId ?? '',
            protocol: this.publicProtocol,
            port: this.publicExternalPort,
            externalPort: this.publicExternalPort,
            status: this.editingPublicMapping.status,
          });
          Object.assign(this.editingPublicMapping, this.mapPublicMapping(updated));
        } catch {
          this.editingPublicMapping.alias = alias;
          this.editingPublicMapping.publicDomain = publicDomain;
          this.editingPublicMapping.sourceRecord = device?.alias || device?.deviceId || '';
          this.editingPublicMapping.deviceId = device?.deviceId ?? '';
          this.editingPublicMapping.protocol = this.publicProtocol;
          this.editingPublicMapping.port = this.publicExternalPort;
          this.editingPublicMapping.externalPort = this.publicExternalPort;
          this.editingPublicMapping.accessMode = this.publicAccessMode;
          this.editingPublicMapping.tlsMode = this.publicTlsMode;
        }
      } else {
        this.editingPublicMapping.alias = alias;
        this.editingPublicMapping.publicDomain = publicDomain;
        this.editingPublicMapping.sourceRecord = device?.alias || device?.deviceId || '';
        this.editingPublicMapping.deviceId = device?.deviceId ?? '';
        this.editingPublicMapping.protocol = this.publicProtocol;
        this.editingPublicMapping.port = this.publicExternalPort;
        this.editingPublicMapping.externalPort = this.publicExternalPort;
        this.editingPublicMapping.accessMode = this.publicAccessMode;
        this.editingPublicMapping.tlsMode = this.publicTlsMode;
      }
      this.closePublicMappingDialog();
      return;
    }
    try {
      const created = await this.api.post<ApiPublicMapping>(WEB_API.publicMappings(this.selectedWorkspaceId), {
        alias,
        publicDomain,
        sourceRecord: device?.alias || device?.deviceId || '',
        deviceId: device?.deviceId ?? '',
        protocol: this.publicProtocol,
        port: this.publicExternalPort,
        externalPort: this.publicExternalPort,
        status: 'enabled',
      });
      this.publicMappings = [...this.publicMappings, this.mapPublicMapping(created)];
    } catch {
      this.publicMappings = [
        ...this.publicMappings,
        { mappingId: this.localResourceId('mapping'), networkId: this.selectedWorkspaceId, workspaceId: this.selectedWorkspaceId, alias, publicDomain, sourceRecord: device?.alias || device?.deviceId || '', deviceId: device?.deviceId ?? '', protocol: this.publicProtocol, port: this.publicExternalPort, externalPort: this.publicExternalPort, accessMode: this.publicAccessMode, tlsMode: this.publicTlsMode, status: 'enabled' },
      ];
    }
    this.closePublicMappingDialog();
  }

  async removePublicMapping(mapping: PublicMappingRow): Promise<void> {
    const mappingId = this.resourceId(mapping.mappingId);
    if (mappingId) {
      try {
        await this.api.delete(WEB_API.publicMapping(mapping.workspaceId, mappingId));
      } catch {
        // Local preview mode removes below.
      }
    }
    this.publicMappings = this.publicMappings.filter((item) => item !== mapping);
  }

  protected resourceId(value: string | undefined): string | undefined {
    const id = value?.trim() ?? '';
    return id || undefined;
  }

  protected localResourceId(prefix: string): string {
    return `local-${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  }
}
