import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, Output } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Network, NetworkAssignment, NetworkDetail, NetworkMember } from './api-contracts';

@Component({
  selector: 'slan-network-workspace',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <section class="network-admin">
      <section class="admin-card" *ngIf="detail?.ownedByCurrentUser && pendingMembers.length > 0">
        <div class="card-title">
          <h3>待确认入网</h3>
          <p>通过邀请码提交的设备需要审核后才会启用网络并分配虚拟 IP。</p>
        </div>
        <div class="table-shell">
          <table>
            <thead><tr><th>Device</th><th>Role</th><th>Status</th><th>Created</th><th class="actions-col"></th></tr></thead>
            <tbody>
              <tr *ngFor="let item of pagedPendingMembers()">
                <td><strong>{{ item.deviceId }}</strong></td>
                <td><span class="status-badge" [attr.data-tone]="roleTone(item.role)">{{ roleLabel(item.role) }}</span></td>
                <td><span class="status-badge" data-tone="warn">{{ memberStatusLabel(item.status) }}</span></td>
                <td>{{ formatMemberTime(item.createdAt) }}</td>
                <td class="row-actions">
                  <button class="primary" type="button" (click)="updateMemberStatus.emit({ memberId: item.memberId, status: 'active' })" [disabled]="isMemberActionBusy(item.memberId)">
                    {{ isMemberActionBusy(item.memberId, 'active') ? '处理中...' : '通过' }}
                  </button>
                  <button type="button" (click)="updateMemberStatus.emit({ memberId: item.memberId, status: 'rejected' })" [disabled]="isMemberActionBusy(item.memberId)">
                    {{ isMemberActionBusy(item.memberId, 'rejected') ? '处理中...' : '拒绝' }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
          <div class="pager" *ngIf="pendingMembers.length > pageSize">
            <span>{{ pageSummary(pendingPage, pendingMembers.length) }}</span>
            <button type="button" (click)="previousPendingPage()" [disabled]="pendingPage <= 1">上一页</button>
            <button type="button" (click)="nextPendingPage()" [disabled]="pendingPage >= totalPages(pendingMembers.length)">下一页</button>
          </div>
        </div>
      </section>
      <section class="admin-card" *ngIf="detail?.ownedByCurrentUser">
        <div class="card-title">
          <h3>地址设备绑定与网络状态</h3>
          <p>以列表形式维护设备地址绑定、接入状态和备注，方便快速核对当前网络内的终端。</p>
        </div>
        <div class="table-shell" *ngIf="assignments.length > 0; else noAssignments">
          <table>
            <thead><tr><th>Virtual IP</th><th>User</th><th>操作系统/版本</th><th>连接方式</th><th>Network Status</th><th>Remark</th><th class="edit-col"></th></tr></thead>
            <tbody>
              <tr *ngFor="let item of assignments">
                <td>{{ draftIps[item.attachmentId] || item.virtualIp || '-' }}</td>
                <td>{{ item.userEmail }}</td>
                <td>{{ assignmentOsVersion(item) }}</td>
                <td>{{ assignmentConnectionType(item) }}</td>
                <td>
                  <span class="status-badge" [attr.data-tone]="assignmentStatusTone(item)">
                    {{ assignmentStatusLabel(item) }}
                  </span>
                </td>
                <td class="remark-text">{{ draftRemarks[item.attachmentId] || item.remark || '-' }}</td>
                <td class="row-actions edit-cell">
                  <button class="edit-icon-button" type="button" (click)="openEditDialog(item)" title="修改" aria-label="修改">
                    ✎
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <ng-template #noAssignments><p class="hint">当前还没有设备挂载记录。</p></ng-template>
      </section>

      <section class="edit-backdrop" *ngIf="editingAssignment" (click)="closeEditDialog()">
        <article class="edit-dialog" (click)="$event.stopPropagation()">
          <div class="edit-title">
            <div>
              <h3>修改设备绑定</h3>
              <p>{{ editingAssignment.deviceId }}</p>
            </div>
            <button type="button" (click)="closeEditDialog()">×</button>
          </div>
          <div class="edit-form">
            <label>
              <span>Virtual IP</span>
              <input [(ngModel)]="editVirtualIp" placeholder="10.0.0.x" />
            </label>
            <label>
              <span>Remark</span>
              <input [(ngModel)]="editRemark" placeholder="客厅主机 / laptop" />
            </label>
          </div>
          <div class="edit-actions">
            <button type="button" (click)="closeEditDialog()">取消</button>
            <button class="primary" type="button" (click)="saveEditDialog()">保存</button>
          </div>
        </article>
      </section>
    </section>
  `,
  styles: [`
    .network-admin { display: grid; gap: 16px; }
    .admin-card { border: 1px solid rgba(15, 23, 42, .09); border-radius: 8px; background: rgba(255, 255, 255, .92); box-shadow: 0 14px 40px rgba(15, 23, 42, .07); padding: 18px; }
    h3, p { margin-top: 0; }
    p, .hint { color: #6b7280; line-height: 1.55; }
    input { width: 100%; border: 1px solid rgba(15, 23, 42, .13); border-radius: 8px; padding: 8px 10px; }
    button { border: 1px solid rgba(15, 23, 42, .12); border-radius: 8px; padding: 8px 12px; color: #111827; background: #fff; cursor: pointer; }
    button.primary { color: #fff; border-color: #ff6900; background: #ff6900; font-weight: 900; }
    button:disabled { cursor: wait; opacity: .62; }
    .row-actions { display: flex; flex-wrap: wrap; gap: 8px; }
    .table-shell { overflow: auto; border: 1px solid rgba(15, 23, 42, .08); border-radius: 8px; }
    table { width: 100%; min-width: 820px; border-collapse: collapse; table-layout: fixed; }
    th, td { padding: 12px; border-bottom: 1px solid rgba(15, 23, 42, .08); text-align: left; vertical-align: top; }
    th { color: #6b7280; font-size: 12px; text-transform: uppercase; background: #f8fafc; }
    tr:last-child td { border-bottom: 0; }
    td strong { display: block; overflow-wrap: anywhere; }
    .remark-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .edit-col, .edit-cell { width: 58px; }
    .actions-col { width: 168px; }
    .edit-icon-button { display: inline-grid; place-items: center; width: 34px; height: 34px; padding: 0; font-size: 17px; line-height: 1; }
    .status-badge { display: inline-flex; align-items: center; border-radius: 999px; padding: 5px 9px; font-size: 12px; font-weight: 800; }
    .status-badge[data-tone="success"] { color: #14532d; background: #dcfce7; }
    .status-badge[data-tone="info"] { color: #1e3a8a; background: #dbeafe; }
    .status-badge[data-tone="warn"] { color: #92400e; background: #fef3c7; }
    .status-badge[data-tone="danger"] { color: #991b1b; background: #fee2e2; }
    .status-badge[data-tone="muted"] { color: #4b5563; background: #f3f4f6; }
    .edit-backdrop { position: fixed; inset: 0; z-index: 60; display: grid; place-items: center; padding: 18px; background: rgba(15, 23, 42, .42); }
    .edit-dialog { width: min(460px, 100%); border: 1px solid rgba(15, 23, 42, .1); border-radius: 12px; background: #fff; box-shadow: 0 24px 70px rgba(15, 23, 42, .22); }
    .edit-title { display: flex; align-items: start; justify-content: space-between; gap: 12px; padding: 18px 20px; border-bottom: 1px solid rgba(15, 23, 42, .08); }
    .edit-title h3 { margin-bottom: 4px; }
    .edit-title p { margin-bottom: 0; overflow-wrap: anywhere; }
    .edit-form { display: grid; gap: 14px; padding: 18px 20px; }
    .edit-form label { display: grid; gap: 8px; color: #334155; font-size: 12px; font-weight: 800; }
    .edit-actions { display: grid; grid-template-columns: 1fr 1.2fr; gap: 12px; padding: 16px 20px 20px; border-top: 1px solid rgba(15, 23, 42, .08); background: #f8fafc; }
    .pager { display: flex; align-items: center; justify-content: flex-end; gap: 8px; padding: 10px 12px; color: #64748b; font-size: 13px; font-weight: 800; border-top: 1px solid rgba(15, 23, 42, .08); }
    .pager button { padding: 7px 11px; font-size: 12px; }
  `]
})
export class NetworkWorkspaceComponent {
  readonly pageSize = 10;
  @Input({ required: true }) activeNetwork!: Network | undefined;
  @Input({ required: true }) detail!: NetworkDetail | null;
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
  @Output() readonly saveAttachmentEdit = new EventEmitter<{ attachmentId: string; virtualIp: string; remark: string }>();
  @Output() readonly updateMemberStatus = new EventEmitter<{ memberId: string; status: 'active' | 'rejected' }>();

  editingAssignment: NetworkAssignment | null = null;
  editVirtualIp = '';
  editRemark = '';
  pendingPage = 1;

  pagedPendingMembers(): NetworkMember[] {
    const page = Math.min(Math.max(1, this.pendingPage), this.totalPages(this.pendingMembers.length));
    const start = (page - 1) * this.pageSize;
    return this.pendingMembers.slice(start, start + this.pageSize);
  }

  pageSummary(page: number, total: number): string {
    if (!total) {
      return '0 / 0';
    }
    const current = Math.min(Math.max(1, page), this.totalPages(total));
    const start = (current - 1) * this.pageSize + 1;
    const end = Math.min(current * this.pageSize, total);
    return `${start}-${end} / ${total}`;
  }

  totalPages(total: number): number {
    return Math.max(1, Math.ceil(total / this.pageSize));
  }

  previousPendingPage(): void {
    this.pendingPage = Math.max(1, this.pendingPage - 1);
  }

  nextPendingPage(): void {
    this.pendingPage = Math.min(this.totalPages(this.pendingMembers.length), this.pendingPage + 1);
  }

  openEditDialog(item: NetworkAssignment): void {
    this.editingAssignment = item;
    this.editVirtualIp = this.draftIps[item.attachmentId] || item.virtualIp || '';
    this.editRemark = this.draftRemarks[item.attachmentId] || item.remark || '';
  }

  closeEditDialog(): void {
    this.editingAssignment = null;
    this.editVirtualIp = '';
    this.editRemark = '';
  }

  saveEditDialog(): void {
    const item = this.editingAssignment;
    if (!item) {
      return;
    }
    this.saveAttachmentEdit.emit({
      attachmentId: item.attachmentId,
      virtualIp: this.editVirtualIp,
      remark: this.editRemark,
    });
    this.closeEditDialog();
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
        return '待确认';
      case 'rejected':
        return '已拒绝';
      default:
        return status || '-';
    }
  }

  assignmentOsVersion(item: NetworkAssignment): string {
    const platform = (item.devicePlatform || '').trim();
    const version = (item.deviceVersion || '').trim();
    if (platform && version) {
      return `${platform} / ${version}`;
    }
    return platform || version || '-';
  }

  assignmentConnectionType(item: NetworkAssignment): string {
    const explicit = (item.connectionType || '').trim().toLowerCase();
    if (explicit === 'app' || explicit === 'console') {
      return explicit;
    }
    return (item.devicePlatform || '').trim().toLowerCase() === 'web' ? 'console' : 'app';
  }

  assignmentStatusLabel(item: NetworkAssignment): string {
    const status = (item.status || '').toLowerCase();
    if (status === 'pending') {
      return '待确认';
    }
    if (status === 'rejected') {
      return '已拒绝';
    }
    if (!item.virtualIp) {
      return status || '-';
    }
    if (!item.runtimeStateFresh || !item.runtimeNetworkOnline) {
      return '离线 / 未应用';
    }
    if (item.runtimeTunnelUp && item.runtimeVirtualIp === item.virtualIp) {
      return '已应用';
    }
    return '已分配';
  }

  assignmentStatusTone(item: NetworkAssignment): string {
    const status = (item.status || '').toLowerCase();
    if (status === 'pending') {
      return 'warn';
    }
    if (status === 'rejected') {
      return 'danger';
    }
    if (!item.virtualIp) {
      return 'muted';
    }
    if (!item.runtimeStateFresh || !item.runtimeNetworkOnline) {
      return 'muted';
    }
    if (item.runtimeTunnelUp && item.runtimeVirtualIp === item.virtualIp) {
      return 'success';
    }
    return 'warn';
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
