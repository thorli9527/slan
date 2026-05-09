import { CommonModule } from '@angular/common';
import { Component, input, output } from '@angular/core';
import { OpsDevice } from '../../shared/models';

@Component({
  selector: 'slan-devices-view',
  standalone: true,
  imports: [CommonModule],
  template: `
    <section class="panel">
      <table>
        <thead><tr><th>设备</th><th>设备 ID</th><th>用户</th><th>平台</th><th>状态</th><th>节点</th></tr></thead>
        <tbody>
          <tr *ngFor="let device of devices()">
            <td>{{ device.name }}</td>
            <td><code>{{ device.deviceId }}</code></td>
            <td><code>{{ device.userId }}</code></td>
            <td>{{ device.platform }}</td>
            <td><span class="pill">{{ device.status }}</span></td>
            <td>{{ device.nodeCount }}</td>
          </tr>
        </tbody>
      </table>
      <div class="pager" *ngIf="filteredCount() > pageSize()">
        <span>{{ pageSummary() }}</span>
        <button type="button" (click)="previousPage.emit()" [disabled]="!canPrevious()">上一页</button>
        <button type="button" (click)="nextPage.emit()" [disabled]="!canNext()">下一页</button>
      </div>
    </section>
  `,
})
export class DevicesViewComponent {
  readonly devices = input.required<OpsDevice[]>();
  readonly filteredCount = input(0);
  readonly pageSize = input(10);
  readonly pageSummary = input('');
  readonly canPrevious = input(false);
  readonly canNext = input(false);
  readonly previousPage = output<void>();
  readonly nextPage = output<void>();
}
