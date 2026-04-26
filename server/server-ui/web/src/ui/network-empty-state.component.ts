import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, Output } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'slan-network-empty-state',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <section class="empty-network-panel">
      <div class="empty-copy">
        <p class="eyebrow">网络初始化</p>
        <h2>当前还没有活动网络</h2>
        <p>先创建自己的网络，或加入已有 owner 的网络。完成后就可以管理子网、设备 IP、设备备注和 Join Key。</p>
      </div>

      <div class="empty-grid">
        <article class="empty-card">
          <h3>创建我的网络</h3>
          <label>
            <span>网络名称</span>
            <input [ngModel]="createName" (ngModelChange)="createNameChange.emit($event)" placeholder="My Network" />
          </label>
          <label>
            <span>描述</span>
            <input [ngModel]="createDescription" (ngModelChange)="createDescriptionChange.emit($event)" placeholder="Personal host network" />
          </label>
          <label>
            <span>默认 CIDR</span>
            <input [ngModel]="createCidr" (ngModelChange)="createCidrChange.emit($event)" placeholder="10.0.0.0/16" />
          </label>
          <p>创建成功后会同步创建默认子网和 DHCP 规划，并立即绑定当前设备。</p>
          <button type="button" (click)="createNetwork.emit()">创建网络</button>
        </article>

        <article class="empty-card">
          <h3>加入别人网络</h3>
          <label>
            <span>Owner 邮箱</span>
            <input [ngModel]="joinOwnerEmail" (ngModelChange)="joinOwnerEmailChange.emit($event)" placeholder="owner@company.com" />
          </label>
          <label>
            <span>设备别名</span>
            <input [ngModel]="joinAlias" (ngModelChange)="joinAliasChange.emit($event)" placeholder="客厅主机 / Thor laptop" />
          </label>
          <p>加入后当前活动网络会切换到目标网络，当前设备会绑定到默认子网。</p>
          <button type="button" (click)="joinNetwork.emit()">提交加入申请</button>
        </article>
      </div>
    </section>
  `,
  styles: [`
    .empty-network-panel {
      display: grid;
      gap: 18px;
      padding: 22px;
      border: 1px solid rgba(15, 23, 42, .09);
      border-radius: 18px;
      background: rgba(255, 255, 255, .92);
      box-shadow: 0 14px 40px rgba(15, 23, 42, .07);
    }
    .eyebrow { margin: 0 0 6px; color: #6b7280; font-size: 12px; font-weight: 800; text-transform: uppercase; }
    h2, h3, p { margin-top: 0; }
    .empty-copy p:last-child, .empty-card p { color: #6b7280; line-height: 1.55; }
    .empty-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
    .empty-card { padding: 18px; border: 1px solid rgba(15, 23, 42, .08); border-radius: 16px; background: #fff; }
    label { display: grid; gap: 8px; margin-bottom: 12px; color: #6b7280; font-size: 13px; font-weight: 800; }
    input { width: 100%; border: 1px solid rgba(15, 23, 42, .13); border-radius: 12px; padding: 12px 13px; }
    button { width: 100%; border: 1px solid #ff6900; border-radius: 12px; padding: 12px 14px; color: #fff; background: #ff6900; font-weight: 900; cursor: pointer; }
    @media (max-width: 760px) { .empty-grid { grid-template-columns: 1fr; } }
  `]
})
export class NetworkEmptyStateComponent {
  @Input({ required: true }) createName!: string;
  @Input({ required: true }) createDescription!: string;
  @Input({ required: true }) createCidr!: string;
  @Input({ required: true }) joinOwnerEmail!: string;
  @Input({ required: true }) joinAlias!: string;

  @Output() readonly createNameChange = new EventEmitter<string>();
  @Output() readonly createDescriptionChange = new EventEmitter<string>();
  @Output() readonly createCidrChange = new EventEmitter<string>();
  @Output() readonly joinOwnerEmailChange = new EventEmitter<string>();
  @Output() readonly joinAliasChange = new EventEmitter<string>();
  @Output() readonly createNetwork = new EventEmitter<void>();
  @Output() readonly joinNetwork = new EventEmitter<void>();
}
