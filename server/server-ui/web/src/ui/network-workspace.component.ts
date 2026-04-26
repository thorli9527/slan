import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, Output } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Network, NetworkAssignment, NetworkDetail, NetworkMember, Subnet } from './api-contracts';

@Component({
  selector: 'slan-network-workspace',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <section class="network-admin">
      <div class="admin-header">
        <div>
          <p class="eyebrow">网络配置</p>
          <h2>{{ activeNetwork?.name || 'Network' }}</h2>
          <p>{{ activeNetwork?.defaultSubnetCidr || '未设置默认 CIDR' }}</p>
        </div>
        <div class="actions">
          <button type="button" *ngIf="showSwitchToOwned" (click)="switchToOwned.emit()">切回我的网络</button>
          <button type="button" (click)="refresh.emit()">刷新</button>
        </div>
      </div>

      <div class="admin-grid three">
        <article class="metric">
          <span>网络 ID</span>
          <strong>{{ activeNetwork?.networkId || '-' }}</strong>
        </article>
        <article class="metric">
          <span>当前设备</span>
          <strong>{{ selectedDeviceLabel || '-' }}</strong>
        </article>
        <article class="metric">
          <span>管理权限</span>
          <strong>{{ detail?.ownedByCurrentUser ? 'Owner' : 'Member' }}</strong>
        </article>
      </div>

      <section class="admin-grid two">
        <article class="admin-card">
          <h3>当前网络</h3>
          <dl>
            <div><dt>描述</dt><dd>{{ activeNetwork?.description || '-' }}</dd></div>
            <div><dt>默认 CIDR</dt><dd>{{ activeNetwork?.defaultSubnetCidr || '-' }}</dd></div>
            <div><dt>Join Key</dt><dd>{{ detail?.joinKeyConfigured || detail?.joinKey ? '已配置' : '未配置' }}</dd></div>
          </dl>
        </article>

        <article class="admin-card">
          <h3>加入 / 切换</h3>
          <label>
            <span>Owner 邮箱</span>
            <input [ngModel]="joinOwnerEmail" (ngModelChange)="joinOwnerEmailChange.emit($event)" placeholder="owner@company.com" />
          </label>
          <label>
            <span>设备别名</span>
            <input [ngModel]="joinAlias" (ngModelChange)="joinAliasChange.emit($event)" placeholder="客厅主机 / Thor laptop" />
          </label>
          <div class="actions">
            <button type="button" (click)="joinByOwnerEmail.emit()">加入 / 切换</button>
            <button type="button" *ngIf="showSwitchToOwned" (click)="switchToOwned.emit()">回到我的网络</button>
          </div>
        </article>
      </section>

      <section class="admin-card" *ngIf="detail?.ownedByCurrentUser">
        <div class="card-title">
          <h3>基础设置</h3>
          <p>修改网络名称、描述和默认网段。客户端收到同步后需要重新应用网络计划。</p>
        </div>
        <div class="form-grid three">
          <label>
            <span>名称</span>
            <input [ngModel]="updateName" (ngModelChange)="updateNameChange.emit($event)" placeholder="Network name" />
          </label>
          <label>
            <span>描述</span>
            <input [ngModel]="updateDescription" (ngModelChange)="updateDescriptionChange.emit($event)" placeholder="Description" />
          </label>
          <label>
            <span>默认 CIDR</span>
            <input [ngModel]="updateCidr" (ngModelChange)="updateCidrChange.emit($event)" placeholder="10.0.0.0/16" />
          </label>
        </div>
        <button type="button" class="primary" (click)="saveNetwork.emit()">保存网络</button>
      </section>

      <section class="admin-card" *ngIf="detail?.ownedByCurrentUser">
        <div class="card-title">
          <h3>子网 / DHCP</h3>
          <p>创建额外子网并维护网关和地址池。</p>
        </div>
        <div class="admin-grid two">
          <div>
            <h4>创建子网</h4>
            <div class="form-grid">
              <label><span>名称</span><input [ngModel]="subnetName" (ngModelChange)="subnetNameChange.emit($event)" placeholder="branch-a" /></label>
              <label><span>CIDR</span><input [ngModel]="subnetCidr" (ngModelChange)="subnetCidrChange.emit($event)" placeholder="10.0.10.0/24" /></label>
              <label><span>网关 IP</span><input [ngModel]="subnetGatewayIp" (ngModelChange)="subnetGatewayIpChange.emit($event)" placeholder="10.0.10.1" /></label>
              <label><span>起始地址</span><input [ngModel]="subnetAllocationStartIp" (ngModelChange)="subnetAllocationStartIpChange.emit($event)" placeholder="10.0.10.2" /></label>
              <label><span>结束地址</span><input [ngModel]="subnetAllocationEndIp" (ngModelChange)="subnetAllocationEndIpChange.emit($event)" placeholder="10.0.10.254" /></label>
            </div>
            <button type="button" class="primary" (click)="createSubnet.emit()">创建子网</button>
          </div>

          <div>
            <h4>当前子网</h4>
            <div class="table-shell" *ngIf="subnets.length > 0; else noSubnets">
              <table>
                <thead><tr><th>名称</th><th>CIDR</th><th>网关</th><th>DHCP</th></tr></thead>
                <tbody>
                  <tr *ngFor="let item of subnets">
                    <td>{{ item.name }} <span class="badge" *ngIf="item.isDefault">default</span></td>
                    <td>{{ item.cidr }}</td>
                    <td>{{ item.gatewayIp || '-' }}</td>
                    <td>{{ formatDhcpRange(item) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <ng-template #noSubnets><p class="hint">当前还没有子网。</p></ng-template>
          </div>
        </div>
      </section>

      <section class="admin-card" *ngIf="detail?.ownedByCurrentUser">
        <div class="card-title">
          <h3>加入申请</h3>
          <p>新设备需要 owner 审批后才能启用网络并分配虚拟 IP。</p>
        </div>
        <div class="table-shell" *ngIf="pendingMembers.length > 0; else noPending">
          <table>
            <thead><tr><th>Device</th><th>Role</th><th>Status</th><th>Created</th><th></th></tr></thead>
            <tbody>
              <tr *ngFor="let item of pendingMembers">
                <td>{{ item.deviceId }}</td>
                <td><span class="status-badge" [attr.data-tone]="roleTone(item.role)">{{ roleLabel(item.role) }}</span></td>
                <td><span class="status-badge" data-tone="warn">{{ memberStatusLabel(item.status) }}</span></td>
                <td>{{ formatMemberTime(item.createdAt) }}</td>
                <td class="row-actions">
                  <button type="button" (click)="updateMemberStatus.emit({ memberId: item.memberId, status: 'active' })" [disabled]="isMemberActionBusy(item.memberId)">
                    {{ isMemberActionBusy(item.memberId, 'active') ? '处理中...' : '通过' }}
                  </button>
                  <button type="button" (click)="updateMemberStatus.emit({ memberId: item.memberId, status: 'rejected' })" [disabled]="isMemberActionBusy(item.memberId)">
                    {{ isMemberActionBusy(item.memberId, 'rejected') ? '处理中...' : '拒绝' }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <ng-template #noPending><p class="hint">当前没有待审批申请。</p></ng-template>
      </section>

      <section class="admin-card" *ngIf="detail?.ownedByCurrentUser">
        <div class="card-title">
          <h3>设备 IP 与备注</h3>
          <p>维护成员设备的虚拟 IP 和别名备注，方便像路由器终端列表一样识别设备。</p>
        </div>
        <div class="table-shell" *ngIf="assignments.length > 0; else noAssignments">
          <table>
            <thead><tr><th>User</th><th>Device</th><th>Role</th><th>Remark</th><th>Subnet</th><th>Virtual IP</th><th></th></tr></thead>
            <tbody>
              <tr *ngFor="let item of assignments">
                <td>{{ item.userEmail }}</td>
                <td>{{ item.deviceName }}</td>
                <td><span class="status-badge" [attr.data-tone]="roleTone(item.role)">{{ roleLabel(item.role) }}</span></td>
                <td>
                  <input [ngModel]="draftRemarks[item.attachmentId] || item.remark || ''" (ngModelChange)="draftRemarkChange.emit({ attachmentId: item.attachmentId, value: $event })" placeholder="客厅主机 / laptop" />
                </td>
                <td>{{ subnetNameById(item.subnetId) }}</td>
                <td>
                  <input [ngModel]="draftIps[item.attachmentId] || item.virtualIp || ''" (ngModelChange)="draftIpChange.emit({ attachmentId: item.attachmentId, value: $event })" placeholder="10.0.0.x" />
                </td>
                <td class="row-actions">
                  <button type="button" (click)="saveAttachmentIp.emit(item.attachmentId)">保存 IP</button>
                  <button type="button" (click)="saveAttachmentRemark.emit(item.attachmentId)">保存备注</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <ng-template #noAssignments><p class="hint">当前还没有设备挂载记录。</p></ng-template>
      </section>
    </section>
  `,
  styles: [`
    .network-admin { display: grid; gap: 16px; }
    .admin-header, .admin-card, .metric { border: 1px solid rgba(15, 23, 42, .09); border-radius: 16px; background: rgba(255, 255, 255, .92); box-shadow: 0 14px 40px rgba(15, 23, 42, .07); }
    .admin-header, .admin-card, .metric { padding: 18px; }
    .admin-header { display: flex; justify-content: space-between; gap: 16px; align-items: start; }
    .eyebrow { margin: 0 0 6px; color: #6b7280; font-size: 12px; font-weight: 800; text-transform: uppercase; }
    h2, h3, h4, p { margin-top: 0; }
    p, .hint { color: #6b7280; line-height: 1.55; }
    .admin-grid, .form-grid { display: grid; gap: 14px; }
    .two { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .three { grid-template-columns: repeat(3, minmax(0, 1fr)); }
    .metric span, dt { color: #6b7280; font-size: 12px; font-weight: 800; }
    .metric strong { display: block; margin-top: 8px; overflow-wrap: anywhere; }
    dl { display: grid; gap: 10px; margin: 0; }
    dl div { display: flex; justify-content: space-between; gap: 12px; padding-bottom: 10px; border-bottom: 1px solid rgba(15, 23, 42, .08); }
    dl div:last-child { border-bottom: 0; padding-bottom: 0; }
    dd { margin: 0; text-align: right; overflow-wrap: anywhere; }
    label { display: grid; gap: 8px; margin-bottom: 12px; color: #6b7280; font-size: 13px; font-weight: 800; }
    input { width: 100%; border: 1px solid rgba(15, 23, 42, .13); border-radius: 12px; padding: 11px 12px; }
    button { border: 1px solid rgba(15, 23, 42, .12); border-radius: 12px; padding: 10px 13px; color: #111827; background: #fff; cursor: pointer; }
    button.primary { color: #fff; border-color: #ff6900; background: #ff6900; font-weight: 900; }
    button:disabled { cursor: wait; opacity: .62; }
    .actions, .row-actions { display: flex; flex-wrap: wrap; gap: 8px; }
    .table-shell { overflow: auto; border: 1px solid rgba(15, 23, 42, .08); border-radius: 14px; }
    table { width: 100%; min-width: 760px; border-collapse: collapse; }
    th, td { padding: 12px; border-bottom: 1px solid rgba(15, 23, 42, .08); text-align: left; vertical-align: top; }
    th { color: #6b7280; font-size: 12px; text-transform: uppercase; background: #f8fafc; }
    tr:last-child td { border-bottom: 0; }
    .badge, .status-badge { display: inline-flex; align-items: center; border-radius: 999px; padding: 5px 9px; font-size: 12px; font-weight: 800; }
    .badge { color: #9a3412; background: rgba(255, 105, 0, .1); }
    .status-badge[data-tone="success"] { color: #14532d; background: #dcfce7; }
    .status-badge[data-tone="info"] { color: #1e3a8a; background: #dbeafe; }
    .status-badge[data-tone="warn"] { color: #92400e; background: #fef3c7; }
    .status-badge[data-tone="danger"] { color: #991b1b; background: #fee2e2; }
    .status-badge[data-tone="muted"] { color: #4b5563; background: #f3f4f6; }
    @media (max-width: 860px) { .admin-header { flex-direction: column; } .two, .three { grid-template-columns: 1fr; } }
  `]
})
export class NetworkWorkspaceComponent {
  @Input({ required: true }) activeNetwork!: Network | undefined;
  @Input({ required: true }) detail!: NetworkDetail | null;
  @Input({ required: true }) subnets!: Subnet[];
  @Input({ required: true }) assignments!: NetworkAssignment[];
  @Input({ required: true }) pendingMembers!: NetworkMember[];
  @Input({ required: true }) selectedDeviceLabel!: string;
  @Input({ required: true }) joinOwnerEmail!: string;
  @Input({ required: true }) joinAlias!: string;
  @Input({ required: true }) updateName!: string;
  @Input({ required: true }) updateDescription!: string;
  @Input({ required: true }) updateCidr!: string;
  @Input({ required: true }) subnetName!: string;
  @Input({ required: true }) subnetCidr!: string;
  @Input({ required: true }) subnetGatewayIp!: string;
  @Input({ required: true }) subnetAllocationStartIp!: string;
  @Input({ required: true }) subnetAllocationEndIp!: string;
  @Input({ required: true }) draftIps!: Record<string, string>;
  @Input({ required: true }) draftRemarks!: Record<string, string>;
  @Input({ required: true }) showSwitchToOwned!: boolean;
  @Input({ required: true }) actionBusy!: string;

  @Output() readonly refresh = new EventEmitter<void>();
  @Output() readonly switchToOwned = new EventEmitter<void>();
  @Output() readonly joinOwnerEmailChange = new EventEmitter<string>();
  @Output() readonly joinAliasChange = new EventEmitter<string>();
  @Output() readonly joinByOwnerEmail = new EventEmitter<void>();
  @Output() readonly updateNameChange = new EventEmitter<string>();
  @Output() readonly updateDescriptionChange = new EventEmitter<string>();
  @Output() readonly updateCidrChange = new EventEmitter<string>();
  @Output() readonly saveNetwork = new EventEmitter<void>();
  @Output() readonly subnetNameChange = new EventEmitter<string>();
  @Output() readonly subnetCidrChange = new EventEmitter<string>();
  @Output() readonly subnetGatewayIpChange = new EventEmitter<string>();
  @Output() readonly subnetAllocationStartIpChange = new EventEmitter<string>();
  @Output() readonly subnetAllocationEndIpChange = new EventEmitter<string>();
  @Output() readonly createSubnet = new EventEmitter<void>();
  @Output() readonly draftIpChange = new EventEmitter<{ attachmentId: string; value: string }>();
  @Output() readonly draftRemarkChange = new EventEmitter<{ attachmentId: string; value: string }>();
  @Output() readonly saveAttachmentIp = new EventEmitter<string>();
  @Output() readonly saveAttachmentRemark = new EventEmitter<string>();
  @Output() readonly updateMemberStatus = new EventEmitter<{ memberId: string; status: 'active' | 'rejected' }>();

  subnetNameById(subnetId: string): string {
    const subnet = this.subnets.find((item) => item.subnetId === subnetId);
    return subnet ? subnet.name : subnetId;
  }

  formatDhcpRange(subnet: Subnet): string {
    if (!subnet.allocationStartIp || !subnet.allocationEndIp) {
      return '-';
    }
    return `${subnet.allocationStartIp} - ${subnet.allocationEndIp}`;
  }

  roleLabel(role: string): string {
    switch ((role || '').toLowerCase()) {
      case 'owner':
        return 'Owner';
      case 'member':
        return '成员';
      default:
        return role || '-';
    }
  }

  roleTone(role: string): string {
    switch ((role || '').toLowerCase()) {
      case 'owner':
        return 'success';
      case 'member':
        return 'info';
      default:
        return 'muted';
    }
  }

  memberStatusLabel(status?: string): string {
    switch ((status || '').toLowerCase()) {
      case 'active':
        return '已通过';
      case 'pending':
        return '待审批';
      case 'rejected':
        return '已拒绝';
      default:
        return status || '-';
    }
  }

  formatMemberTime(value?: number): string {
    return value ? new Date(value * 1000).toLocaleString() : '-';
  }

  isMemberActionBusy(memberId: string, status?: 'active' | 'rejected'): boolean {
    const busy = this.actionBusy || '';
    if (status) {
      return busy === `member:${memberId}:${status}`;
    }
    return busy.startsWith(`member:${memberId}:`);
  }
}
