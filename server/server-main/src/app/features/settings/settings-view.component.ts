import { CommonModule } from '@angular/common';
import { Component, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { PlanConfig } from '../../shared/models';

@Component({
  selector: 'slan-settings-view',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>全局设备与流量配置</h2>
          <p class="muted">用户未设置专属配置时，统一使用这里的全局限制。</p>
        </div>
        <button type="button" class="primary narrow-primary" (click)="save.emit()" [disabled]="loading()">保存配置</button>
      </div>
      <div class="form-grid">
        <label>
          <span>最大设备数</span>
          <input type="number" min="1" [ngModel]="config().maxActiveDevices" (ngModelChange)="update.emit({ key: 'maxActiveDevices', value: $event })" />
        </label>
        <label>
          <span>转发接收 Kbps</span>
          <input type="number" min="0" [ngModel]="config().relayIngressKbps" (ngModelChange)="update.emit({ key: 'relayIngressKbps', value: $event })" />
        </label>
        <label>
          <span>转发发送 Kbps</span>
          <input type="number" min="0" [ngModel]="config().relayEgressKbps" (ngModelChange)="update.emit({ key: 'relayEgressKbps', value: $event })" />
        </label>
        <label>
          <span>UDP/打洞接收 Kbps</span>
          <input type="number" min="0" [ngModel]="config().udpIngressKbps" (ngModelChange)="update.emit({ key: 'udpIngressKbps', value: $event })" />
        </label>
        <label>
          <span>UDP/打洞发送 Kbps</span>
          <input type="number" min="0" [ngModel]="config().udpEgressKbps" (ngModelChange)="update.emit({ key: 'udpEgressKbps', value: $event })" />
        </label>
      </div>
    </section>
  `,
})
export class SettingsViewComponent {
  readonly config = input.required<PlanConfig>();
  readonly loading = input(false);
  readonly save = output<void>();
  readonly update = output<{ key: keyof PlanConfig; value: number }>();
}
