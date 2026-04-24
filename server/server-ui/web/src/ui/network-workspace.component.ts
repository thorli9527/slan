import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, Output } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Network, NetworkAssignment, NetworkDetail, NetworkMember, Subnet } from './api-contracts';

@Component({
  selector: 'slan-network-workspace',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <section class="panel">
      <div class="panel-title split">
        <div>
          <h2>{{ activeNetwork?.name || 'Network' }}</h2>
          <p>{{ activeNetwork?.defaultSubnetCidr || '未定义 CIDR' }}</p>
        </div>
        <div class="actions compact">
          <button class="ghost" *ngIf="showSwitchToOwned" (click)="switchToOwned.emit()">切回我的网络</button>
          <button class="ghost" (click)="refresh.emit()">刷新</button>
        </div>
      </div>

      <div class="grid three">
        <div class="metric">
          <span class="metric-label">网络 ID</span>
          <strong>{{ activeNetwork?.networkId || '-' }}</strong>
        </div>
        <div class="metric">
          <span class="metric-label">当前设备</span>
          <strong>{{ selectedDeviceLabel || '-' }}</strong>
        </div>
        <div class="metric">
          <span class="metric-label">归我管理</span>
          <strong>{{ detail?.ownedByCurrentUser ? '是' : '否' }}</strong>
        </div>
      </div>

      <div class="grid two">
        <div class="card section-card">
          <h3>当前网络</h3>
          <p><strong>描述：</strong> {{ activeNetwork?.description || '-' }}</p>
          <p><strong>默认 CIDR：</strong> {{ activeNetwork?.defaultSubnetCidr || '-' }}</p>
          <p class="hint">当前用户同一时刻只保留一个活动网络。加入别人的网络或切换网络都会刷新活动网络指针。</p>
        </div>

        <div class="card section-card">
          <h3>加入 / 切换</h3>
          <label>
            <span>通过宿主邮箱加入</span>
            <input [ngModel]="joinOwnerEmail" (ngModelChange)="joinOwnerEmailChange.emit($event)" placeholder="owner@company.com" />
          </label>
          <div class="actions compact">
            <button class="ghost" (click)="joinByOwnerEmail.emit()">加入 / 切换</button>
            <button class="ghost" *ngIf="showSwitchToOwned" (click)="switchToOwned.emit()">回到我的网络</button>
          </div>
        </div>
      </div>

      <div class="card section-card" *ngIf="detail?.ownedByCurrentUser">
        <div class="panel-title">
          <h3>更新我的网络</h3>
          <p>修改自己的默认网段时，服务端会重新分配成员虚拟 IP，并通过 control sync 通知客户端重启网络。</p>
        </div>
        <div class="form-grid three-up">
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
        <div class="actions compact">
          <button class="ghost" (click)="saveNetwork.emit()">保存网络</button>
        </div>
      </div>

      <div class="card section-card" *ngIf="detail?.ownedByCurrentUser">
        <div class="panel-title">
          <h3>子网 / DHCP</h3>
          <p>在自己的网络中创建额外子网，并同步配置 DHCP 范围。表单会先做前端校验，服务端也会再次校验。</p>
        </div>

        <div class="grid two">
          <div>
            <h4>创建子网</h4>
            <div class="form-grid">
              <label>
                <span>名称</span>
                <input [ngModel]="subnetName" (ngModelChange)="subnetNameChange.emit($event)" placeholder="branch-a" />
              </label>
              <label>
                <span>CIDR</span>
                <input [ngModel]="subnetCidr" (ngModelChange)="subnetCidrChange.emit($event)" placeholder="10.0.10.0/24" />
              </label>
              <label>
                <span>网关 IP</span>
                <input [ngModel]="subnetGatewayIp" (ngModelChange)="subnetGatewayIpChange.emit($event)" placeholder="10.0.10.1" />
              </label>
              <label>
                <span>起始地址</span>
                <input [ngModel]="subnetAllocationStartIp" (ngModelChange)="subnetAllocationStartIpChange.emit($event)" placeholder="10.0.10.2" />
              </label>
              <label>
                <span>结束地址</span>
                <input [ngModel]="subnetAllocationEndIp" (ngModelChange)="subnetAllocationEndIpChange.emit($event)" placeholder="10.0.10.254" />
              </label>
            </div>
            <p class="hint">
              规则：allocationStartIp 和 allocationEndIp 必须成对出现；网关、起止地址必须都落在 CIDR 内，且不能是网络地址或广播地址。
            </p>
            <div class="actions compact">
              <button class="ghost" (click)="createSubnet.emit()">创建子网</button>
            </div>
          </div>

          <div>
            <h4>当前子网</h4>
            <table *ngIf="subnets.length > 0">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>CIDR</th>
                  <th>网关</th>
                  <th>DHCP 范围</th>
                </tr>
              </thead>
              <tbody>
                <tr *ngFor="let item of subnets">
                  <td>
                    {{ item.name }}
                    <span class="badge" *ngIf="item.isDefault">default</span>
                  </td>
                  <td>{{ item.cidr }}</td>
                  <td>{{ item.gatewayIp || '-' }}</td>
                  <td>{{ formatDhcpRange(item) }}</td>
                </tr>
              </tbody>
            </table>
            <p class="hint" *ngIf="subnets.length === 0">当前还没有子网。</p>
          </div>
        </div>
      </div>

      <div class="card section-card" *ngIf="detail?.ownedByCurrentUser">
        <div class="panel-title">
          <h3>加入申请</h3>
          <p>申请中的设备需要 owner 审批后才能激活网络并分配虚拟 IP。</p>
        </div>
        <table *ngIf="pendingMembers.length > 0">
          <thead>
            <tr>
              <th>Device</th>
              <th>Role</th>
              <th>Status</th>
              <th>Created</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr *ngFor="let item of pendingMembers">
              <td>{{ item.deviceId }}</td>
              <td><span class="status-badge" [attr.data-tone]="roleTone(item.role)">{{ roleLabel(item.role) }}</span></td>
              <td><span class="status-badge" data-tone="warn">{{ memberStatusLabel(item.status) }}</span></td>
              <td>{{ formatMemberTime(item.createdAt) }}</td>
              <td>
                <button
                  class="ghost"
                  (click)="updateMemberStatus.emit({ memberId: item.memberId, status: 'active' })"
                  [disabled]="isMemberActionBusy(item.memberId)"
                >
                  {{ isMemberActionBusy(item.memberId, 'active') ? '处理中...' : '通过' }}
                </button>
                <button
                  class="ghost"
                  (click)="updateMemberStatus.emit({ memberId: item.memberId, status: 'rejected' })"
                  [disabled]="isMemberActionBusy(item.memberId)"
                >
                  {{ isMemberActionBusy(item.memberId, 'rejected') ? '处理中...' : '拒绝' }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
        <p class="hint" *ngIf="pendingMembers.length === 0">当前没有待审批加入申请。</p>
      </div>

      <div class="card section-card" *ngIf="detail?.ownedByCurrentUser">
        <div class="panel-title">
          <h3>网络 IP 管理</h3>
          <p>只有宿主网络 owner 能管理成员虚拟 IP 和网络内设备备注。owner 设备固定为该默认子网的 x.x.x.2。</p>
        </div>
        <table *ngIf="assignments.length > 0">
          <thead>
            <tr>
              <th>User</th>
              <th>Device</th>
              <th>Role</th>
              <th>Remark</th>
              <th>Subnet</th>
              <th>Virtual IP</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr *ngFor="let item of assignments">
              <td>{{ item.userEmail }}</td>
              <td>{{ item.deviceName }}</td>
              <td><span class="status-badge" [attr.data-tone]="roleTone(item.role)">{{ roleLabel(item.role) }}</span></td>
              <td>
                <input
                  [ngModel]="draftRemarks[item.attachmentId] || item.remark || ''"
                  (ngModelChange)="draftRemarkChange.emit({ attachmentId: item.attachmentId, value: $event })"
                  placeholder="branch router / laptop / camera"
                />
              </td>
              <td>{{ subnetNameById(item.subnetId) }}</td>
              <td>
                <input
                  [ngModel]="draftIps[item.attachmentId] || item.virtualIp || ''"
                  (ngModelChange)="draftIpChange.emit({ attachmentId: item.attachmentId, value: $event })"
                  placeholder="10.0.0.x"
                />
              </td>
              <td>
                <button class="ghost" (click)="saveAttachmentIp.emit(item.attachmentId)">保存 IP</button>
                <button class="ghost" (click)="saveAttachmentRemark.emit(item.attachmentId)">保存备注</button>
              </td>
            </tr>
          </tbody>
        </table>
        <p class="hint" *ngIf="assignments.length === 0">当前还没有设备挂载记录。</p>
      </div>
    </section>
  `
})
export class NetworkWorkspaceComponent {
  @Input({ required: true }) activeNetwork!: Network | undefined;
  @Input({ required: true }) detail!: NetworkDetail | null;
  @Input({ required: true }) subnets!: Subnet[];
  @Input({ required: true }) assignments!: NetworkAssignment[];
  @Input({ required: true }) pendingMembers!: NetworkMember[];
  @Input({ required: true }) selectedDeviceLabel!: string;
  @Input({ required: true }) joinOwnerEmail!: string;
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
        return '宿主';
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
