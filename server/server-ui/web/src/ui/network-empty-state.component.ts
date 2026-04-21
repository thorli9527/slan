import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, Output } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'slan-network-empty-state',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <section class="panel">
      <div class="panel-title">
        <h2>当前没有活动网络</h2>
        <p>先完成网络创建或加入，再进入后续的子网、设备 IP、备注和加入 key 管理。</p>
      </div>

      <div class="grid two">
        <div class="card section-card">
          <h3>创建我的网络</h3>
          <div class="form-grid">
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
          </div>
          <p class="hint">
            创建成功时会一并创建默认子网和 DHCP 规划，并立即绑定当前设备。owner 设备固定优先使用 x.x.x.2。
          </p>
          <button class="ghost" (click)="createNetwork.emit()">创建网络</button>
        </div>

        <div class="card section-card">
          <h3>加入别人的网络</h3>
          <label>
            <span>宿主邮箱</span>
            <input [ngModel]="joinOwnerEmail" (ngModelChange)="joinOwnerEmailChange.emit($event)" placeholder="owner@company.com" />
          </label>
          <p class="hint">加入成功后，当前活动网络会切换到目标宿主网络，当前设备会自动绑定到该网络默认子网。</p>
          <button class="ghost" (click)="joinNetwork.emit()">加入网络</button>
        </div>
      </div>
    </section>
  `
})
export class NetworkEmptyStateComponent {
  @Input({ required: true }) createName!: string;
  @Input({ required: true }) createDescription!: string;
  @Input({ required: true }) createCidr!: string;
  @Input({ required: true }) joinOwnerEmail!: string;

  @Output() readonly createNameChange = new EventEmitter<string>();
  @Output() readonly createDescriptionChange = new EventEmitter<string>();
  @Output() readonly createCidrChange = new EventEmitter<string>();
  @Output() readonly joinOwnerEmailChange = new EventEmitter<string>();
  @Output() readonly createNetwork = new EventEmitter<void>();
  @Output() readonly joinNetwork = new EventEmitter<void>();
}
