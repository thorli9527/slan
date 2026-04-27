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
          <p>加入新的网络统一在对话框里填写 owner 邮箱、Join Key 和设备别名。</p>
          <div class="actions">
            <button type="button" (click)="openJoinNetwork.emit()">加入别人的网络</button>
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
          <h3>子网与 DHCP</h3>
          <p>当前版本不再从控制台创建子网，只展示网络创建时生成的默认地址池和网关。</p>
        </div>
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
          <h3>地址设备绑定与网络状态</h3>
          <p>以列表形式维护设备地址绑定、接入状态和备注，方便快速核对当前网络内的终端。</p>
        </div>
        <div class="table-shell" *ngIf="assignments.length > 0; else noAssignments">
          <table>
            <thead><tr><th>Virtual IP</th><th>Device</th><th>User</th><th>Subnet</th><th>Network Status</th><th>Remark</th><th></th></tr></thead>
            <tbody>
              <tr *ngFor="let item of assignments">
                <td>
                  <input [ngModel]="draftIps[item.attachmentId] || item.virtualIp || ''" (ngModelChange)="draftIpChange.emit({ attachmentId: item.attachmentId, value: $event })" placeholder="10.0.0.x" />
                </td>
                <td>
                  <strong>{{ item.deviceName || item.deviceId }}</strong>
                  <small>{{ item.deviceId }}</small>
                </td>
                <td>{{ item.userEmail }}</td>
                <td>{{ subnetNameById(item.subnetId) }}</td>
                <td>
                  <span class="status-badge" [attr.data-tone]="assignmentStatusTone(item)">
                    {{ assignmentStatusLabel(item) }}
                  </span>
                </td>
                <td>
                  <input [ngModel]="draftRemarks[item.attachmentId] || item.remark || ''" (ngModelChange)="draftRemarkChange.emit({ attachmentId: item.attachmentId, value: $event })" placeholder="客厅主机 / laptop" />
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
    .admin-header, .admin-card, .metric { border: 1px solid rgba(15, 23, 42, .09); border-radius: 8px; background: rgba(255, 255, 255, .92); box-shadow: 0 14px 40px rgba(15, 23, 42, .07); }
    .admin-header, .admin-card, .metric { padding: 18px; }
    .admin-header { display: flex; justify-content: space-between; gap: 16px; align-items: start; }
    .eyebrow { margin: 0 0 6px; color: #6b7280; font-size: 12px; font-weight: 800; text-transform: uppercase; }
    h2, h3, p { margin-top: 0; }
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
    input { width: 100%; border: 1px solid rgba(15, 23, 42, .13); border-radius: 8px; padding: 11px 12px; }
    button { border: 1px solid rgba(15, 23, 42, .12); border-radius: 8px; padding: 10px 13px; color: #111827; background: #fff; cursor: pointer; }
    button.primary { color: #fff; border-color: #ff6900; background: #ff6900; font-weight: 900; }
    button:disabled { cursor: wait; opacity: .62; }
    .actions, .row-actions { display: flex; flex-wrap: wrap; gap: 8px; }
    .table-shell { overflow: auto; border: 1px solid rgba(15, 23, 42, .08); border-radius: 8px; }
    table { width: 100%; min-width: 760px; border-collapse: collapse; }
    th, td { padding: 12px; border-bottom: 1px solid rgba(15, 23, 42, .08); text-align: left; vertical-align: top; }
    th { color: #6b7280; font-size: 12px; text-transform: uppercase; background: #f8fafc; }
    tr:last-child td { border-bottom: 0; }
    td strong, td small { display: block; overflow-wrap: anywhere; }
    td small { margin-top: 4px; color: #6b7280; }
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
  @Input({ required: true }) updateName!: string;
  @Input({ required: true }) updateDescription!: string;
  @Input({ required: true }) updateCidr!: string;
  @Input({ required: true }) draftIps!: Record<string, string>;
  @Input({ required: true }) draftRemarks!: Record<string, string>;
  @Input({ required: true }) showSwitchToOwned!: boolean;
  @Input({ required: true }) actionBusy!: string;

  @Output() readonly refresh = new EventEmitter<void>();
  @Output() readonly switchToOwned = new EventEmitter<void>();
  @Output() readonly openJoinNetwork = new EventEmitter<void>();
  @Output() readonly updateNameChange = new EventEmitter<string>();
  @Output() readonly updateDescriptionChange = new EventEmitter<string>();
  @Output() readonly updateCidrChange = new EventEmitter<string>();
  @Output() readonly saveNetwork = new EventEmitter<void>();
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

  assignmentStatusLabel(item: NetworkAssignment): string {
    const status = (item.status || '').toLowerCase();
    if (status === 'active') {
      return item.virtualIp ? '已接入 / 已绑定' : '已接入';
    }
    if (status === 'pending') {
      return '待审批';
    }
    if (status === 'rejected') {
      return '已拒绝';
    }
    return item.virtualIp ? '已绑定' : (status || '-');
  }

  assignmentStatusTone(item: NetworkAssignment): string {
    const status = (item.status || '').toLowerCase();
    if (status === 'active' || item.virtualIp) {
      return 'success';
    }
    if (status === 'pending') {
      return 'warn';
    }
    if (status === 'rejected') {
      return 'danger';
    }
    return 'muted';
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
