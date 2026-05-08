import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Output } from '@angular/core';

@Component({
  selector: 'slan-network-empty-state',
  standalone: true,
  imports: [CommonModule],
  template: `
    <section class="empty-network-panel">
      <div class="empty-copy">
        <p class="eyebrow">网络初始化</p>
        <h2>当前还没有活动网络</h2>
        <p>可以创建自己的网络，也可以加入别人已经建好的网络。创建和加入都会在对话框里完成，避免误操作。</p>
      </div>

      <div class="empty-grid">
        <button class="empty-card" type="button" (click)="openCreateNetwork.emit()">
          <strong>创建网络</strong>
          <span>设置名称、IP 地址、子网掩码和 DHCP 地址池。</span>
        </button>
        <button class="empty-card" type="button" (click)="openJoinNetwork.emit()">
          <strong>加入别人的网络</strong>
          <span>填写邀请码提交加入申请。</span>
        </button>
      </div>
    </section>
  `,
  styles: [`
    .empty-network-panel {
      display: grid;
      gap: 18px;
      padding: 22px;
      border: 1px solid rgba(15, 23, 42, .09);
      border-radius: 16px;
      background: rgba(255, 255, 255, .92);
      box-shadow: 0 14px 40px rgba(15, 23, 42, .07);
    }
    .eyebrow { margin: 0 0 6px; color: #6b7280; font-size: 12px; font-weight: 800; text-transform: uppercase; }
    h2, p { margin-top: 0; }
    .empty-copy p:last-child { color: #6b7280; line-height: 1.55; }
    .empty-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
    .empty-card {
      display: grid;
      gap: 8px;
      min-height: 118px;
      padding: 18px;
      border: 1px solid rgba(15, 23, 42, .1);
      border-radius: 14px;
      background: #fff;
      color: #111827;
      text-align: left;
      cursor: pointer;
    }
    .empty-card:hover { border-color: rgba(255, 105, 0, .55); box-shadow: 0 12px 30px rgba(15, 23, 42, .08); }
    .empty-card strong { font-size: 18px; }
    .empty-card span { color: #6b7280; line-height: 1.5; }
    @media (max-width: 760px) { .empty-grid { grid-template-columns: 1fr; } }
  `]
})
export class NetworkEmptyStateComponent {
  @Output() readonly openCreateNetwork = new EventEmitter<void>();
  @Output() readonly openJoinNetwork = new EventEmitter<void>();
}
