import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, Output } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Device } from './api-contracts';

@Component({
  selector: 'slan-device-context',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <section class="panel">
      <div class="panel-title split">
        <div>
          <h2>管理设备</h2>
          <p>网页端的加入、切网、IP 绑定和设备管理动作都默认作用在这里选中的设备上。</p>
        </div>
        <div class="actions compact">
          <button class="ghost" (click)="reloadDevices.emit()">刷新设备</button>
          <button class="ghost" (click)="logout.emit()">退出登录</button>
        </div>
      </div>

      <div class="grid three">
        <div class="metric">
          <span class="metric-label">当前设备</span>
          <strong>{{ selectedDeviceLabel || '未选择' }}</strong>
        </div>
        <div class="metric">
          <span class="metric-label">活动网络</span>
          <strong>{{ activeNetworkName || '未接入' }}</strong>
        </div>
        <div class="metric">
          <span class="metric-label">当前角色</span>
          <strong>{{ roleLabel }}</strong>
        </div>
      </div>

      <label class="device-picker">
        <span>切换管理设备</span>
        <select [ngModel]="currentDeviceId" (ngModelChange)="currentDeviceIdChange.emit($event)">
          <option value="">选择设备</option>
          <option *ngFor="let item of devices" [value]="item.deviceId">
            {{ item.name }} · {{ item.deviceId }} · {{ item.platform }}
          </option>
        </select>
      </label>
    </section>
  `
})
export class DeviceContextComponent {
  @Input({ required: true }) selectedDeviceLabel!: string;
  @Input({ required: true }) activeNetworkName!: string;
  @Input({ required: true }) roleLabel!: string;
  @Input({ required: true }) devices!: Device[];
  @Input({ required: true }) currentDeviceId!: string;

  @Output() readonly reloadDevices = new EventEmitter<void>();
  @Output() readonly logout = new EventEmitter<void>();
  @Output() readonly currentDeviceIdChange = new EventEmitter<string>();
}
