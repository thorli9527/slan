import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { PaginationComponent } from '../../shared/pagination.component';
import { DEFAULT_PAGE_SIZE, paginate } from '../../shared/pagination';

@Component({
  selector: 'ops-network-detail-page',
  standalone: true,
  imports: [CommonModule, FormsModule, PaginationComponent],
  templateUrl: './network-detail-page.component.html',
})
export class NetworkDetailPageComponent {
  @Input({ required: true }) vm!: any;

  dnsResource: 'zones' | 'records' = 'zones';
  dnsKeyword = '';
  securityKeyword = '';
  showDNSZoneDialog = false;
  showDNSRecordDialog = false;
  showSecurityGroupDialog = false;
  showSecurityRuleDialog = false;
  pageSize = DEFAULT_PAGE_SIZE;
  dnsZonePage = 1;
  dnsRecordPage = 1;
  securityGroupPage = 1;
  securityRulePage = 1;

  get filteredDNSZones(): any[] {
    const keyword = this.dnsKeyword.trim().toLowerCase();
    if (!keyword) return this.vm.dnsZones;
    return this.vm.dnsZones.filter((item: any) =>
      [item.name, item.zoneId, item.status].some((value) => String(value ?? '').toLowerCase().includes(keyword)),
    );
  }

  get filteredDNSRecords(): any[] {
    const keyword = this.dnsKeyword.trim().toLowerCase();
    if (!keyword) return this.vm.dnsRecords;
    return this.vm.dnsRecords.filter((item: any) =>
      [item.name, item.type, item.value, item.recordId, this.zoneName(item.zoneId)]
        .some((value) => String(value ?? '').toLowerCase().includes(keyword)),
    );
  }

  get filteredSecurityGroups(): any[] {
    const keyword = this.securityKeyword.trim().toLowerCase();
    if (!keyword) return this.vm.securityGroups;
    return this.vm.securityGroups.filter((item: any) =>
      [item.name, item.description, item.securityGroupId]
        .some((value) => String(value ?? '').toLowerCase().includes(keyword)),
    );
  }

  get filteredSecurityRules(): any[] {
    const keyword = this.securityKeyword.trim().toLowerCase();
    const rules = this.vm.securityRules.filter(
      (item: any) => item.securityGroupId === this.vm.securityGroupDetailId,
    );
    if (!keyword) return rules;
    return rules.filter((item: any) =>
      [item.direction, item.protocol, item.portRange, item.peerType, item.peerValue, item.action,
        item.description, this.securityGroupName(item.securityGroupId)]
        .some((value) => String(value ?? '').toLowerCase().includes(keyword)),
    );
  }

  get pagedDNSZones(): any[] { return paginate(this.filteredDNSZones, this.dnsZonePage, this.pageSize); }
  get pagedDNSRecords(): any[] { return paginate(this.filteredDNSRecords, this.dnsRecordPage, this.pageSize); }
  get pagedSecurityGroups(): any[] { return paginate(this.filteredSecurityGroups, this.securityGroupPage, this.pageSize); }
  get pagedSecurityRules(): any[] { return paginate(this.filteredSecurityRules, this.securityRulePage, this.pageSize); }

  get availableNetworkDevices(): any[] {
    const ids = new Set(this.vm.networkDetailNetwork?.deviceIds || []);
    return this.vm.devices.filter((item: any) => ids.has(item.deviceId));
  }

  get availableNetworkDeviceGroups(): any[] {
    const ids = new Set(this.vm.networkDetailNetwork?.deviceGroupIds || []);
    return this.vm.deviceGroups.filter((item: any) => ids.has(item.groupId));
  }

  deviceLabel(device: any): string {
    return device.name || device.deviceId;
  }

  onDNSRecordTypeChange(type: string): void {
    this.vm.dnsRecordForm.value = type === 'CNAME' ? '' : (this.availableNetworkDevices[0]?.deviceId || '');
    this.vm.dnsRecordForm.port = '';
  }

  onSecurityPeerTypeChange(peerType: string): void {
    this.vm.securityRuleForm.peerValue = peerType === 'device'
      ? (this.availableNetworkDevices[0]?.deviceId || '')
      : (this.availableNetworkDeviceGroups[0]?.groupId || '');
  }

  setPolicyTab(tab: 'dns' | 'security'): void {
    this.vm.openNetworkPolicyTab(tab);
    this.dnsKeyword = '';
    this.securityKeyword = '';
    this.resetAllPages();
  }

  setDNSResource(resource: 'zones' | 'records'): void {
    this.dnsResource = resource;
    this.dnsKeyword = '';
    this.resetDNSPage();
  }

  resetDNSPage(): void {
    if (this.dnsResource === 'zones') this.dnsZonePage = 1;
    else this.dnsRecordPage = 1;
  }

  resetSecurityPage(): void {
    if (this.vm.securityGroupDetail) this.securityRulePage = 1;
    else this.securityGroupPage = 1;
  }

  setPageSize(pageSize: number): void {
    this.pageSize = pageSize;
    this.resetAllPages();
  }

  private resetAllPages(): void {
    this.dnsZonePage = 1;
    this.dnsRecordPage = 1;
    this.securityGroupPage = 1;
    this.securityRulePage = 1;
  }

  zoneName(zoneId: string): string {
    return this.vm.dnsZones.find((item: any) => item.zoneId === zoneId)?.name || '-';
  }

  zoneRecordCount(zoneId: string): number {
    return this.vm.dnsRecords.filter((item: any) => item.zoneId === zoneId).length;
  }

  securityGroupName(securityGroupId: string): string {
    return this.vm.securityGroups.find((item: any) => item.securityGroupId === securityGroupId)?.name || '-';
  }

  securityRuleCount(securityGroupId: string): number {
    return this.vm.securityRules.filter((item: any) => item.securityGroupId === securityGroupId).length;
  }

  showRulesForGroup(event: MouseEvent, group: any): void {
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    this.securityKeyword = '';
    this.securityRulePage = 1;
    this.vm.openSecurityGroupRules(group);
  }

  closeSecurityGroupRules(): void {
    this.securityKeyword = '';
    this.securityGroupPage = 1;
    this.vm.closeSecurityGroupRules();
  }

  openDNSZone(item?: any): void {
    if (item) this.vm.editDNSZone(item);
    else {
      this.vm.selectedDNSZone = null;
      this.vm.dnsZoneForm = { name: '', status: 'active' };
    }
    this.vm.apiMessage = '';
    this.showDNSZoneDialog = true;
  }

  openDNSRecord(item?: any): void {
    if (item) this.vm.editDNSRecord(item);
    else {
      this.vm.selectedDNSRecord = null;
      this.vm.dnsRecordForm = {
        zoneId: this.vm.dnsZones[0]?.zoneId || '',
        name: '', type: 'A', value: this.availableNetworkDevices[0]?.deviceId || '', port: '', ttl: 300,
      };
    }
    this.vm.apiMessage = '';
    this.showDNSRecordDialog = true;
  }

  openSecurityGroup(): void {
    this.vm.securityGroupForm = { name: '', description: '' };
    this.vm.apiMessage = '';
    this.showSecurityGroupDialog = true;
  }

  openSecurityRule(item?: any): void {
    if (item) this.vm.editSecurityRule(item);
    else {
      this.vm.selectedSecurityRule = null;
      this.vm.securityRuleForm = {
        securityGroupId: this.vm.securityGroupDetailId || '',
        direction: 'ingress', protocol: 'any', portRange: '', peerType: 'device_group',
        peerValue: this.availableNetworkDeviceGroups[0]?.groupId || '', action: 'allow', priority: 100, description: '', enabled: true,
      };
    }
    this.vm.apiMessage = '';
    this.showSecurityRuleDialog = true;
  }

  async saveDNSZone(): Promise<void> {
    this.vm.apiMessage = '';
    await this.vm.saveDNSZone();
    if (this.vm.apiMessageIsSuccess) this.showDNSZoneDialog = false;
  }

  async saveDNSRecord(): Promise<void> {
    this.vm.apiMessage = '';
    await this.vm.saveDNSRecord();
    if (this.vm.apiMessageIsSuccess) this.showDNSRecordDialog = false;
  }

  async saveSecurityGroup(): Promise<void> {
    this.vm.apiMessage = '';
    await this.vm.saveSecurityGroup();
    if (this.vm.apiMessageIsSuccess) this.showSecurityGroupDialog = false;
  }

  async saveSecurityRule(): Promise<void> {
    this.vm.apiMessage = '';
    await this.vm.saveSecurityRule();
    if (this.vm.apiMessageIsSuccess) this.showSecurityRuleDialog = false;
  }
}
