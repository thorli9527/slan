import { CommonModule } from '@angular/common';
import { Component, input, output } from '@angular/core';
import { OpsNetworkQualityItem } from '../../shared/models';

@Component({
  selector: 'slan-quality-view',
  standalone: true,
  imports: [CommonModule],
  template: `
    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>用户实时网络质量</h2>
          <p class="muted">展示最近 30 分钟客户端上报的路径质量样本。</p>
        </div>
        <button type="button" (click)="refresh.emit()" [disabled]="loading()">刷新质量</button>
      </div>
      <table>
        <thead><tr><th>质量</th><th>用户/设备</th><th>网络</th><th>路径</th><th>RTT</th><th>丢包</th><th>评分</th><th>时间</th></tr></thead>
        <tbody>
          <tr *ngFor="let item of items()">
            <td><span [class]="qualityTone(item)">{{ qualityLabel(item) }}</span></td>
            <td>
              <strong>{{ item.userEmail || item.userId || '-' }}</strong>
              <small>{{ item.deviceName || item.deviceId || '-' }}</small>
            </td>
            <td>
              {{ item.networkName || item.networkId }}
              <small><code>{{ item.networkId }}</code></small>
            </td>
            <td>
              {{ item.pathType || '-' }}
              <small>{{ item.derpNodeId || item.endpoint || item.peerNodeId || '-' }}</small>
            </td>
            <td>{{ item.observedRttMs ?? '-' }}</td>
            <td>{{ item.packetLossPpm ?? '-' }}</td>
            <td>{{ item.pathScore ?? '-' }}</td>
            <td>
              {{ formatTime(item.updatedAt) }}
              <small>{{ formatSampleTime(item.sampledAtMs) }}</small>
            </td>
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
export class QualityViewComponent {
  readonly items = input.required<OpsNetworkQualityItem[]>();
  readonly filteredCount = input(0);
  readonly pageSize = input(10);
  readonly pageSummary = input('');
  readonly canPrevious = input(false);
  readonly canNext = input(false);
  readonly loading = input(false);
  readonly refresh = output<void>();
  readonly previousPage = output<void>();
  readonly nextPage = output<void>();

  qualityTone(item: OpsNetworkQualityItem): string {
    const loss = item.packetLossPpm ?? 0;
    const rtt = item.observedRttMs ?? 0;
    const score = item.pathScore ?? 0;
    if (loss > 50_000 || rtt > 500 || score > 2000) {
      return 'status-bad';
    }
    if (loss > 0 || rtt > 180 || score > 900) {
      return 'status-warn';
    }
    return 'status-ok';
  }

  qualityLabel(item: OpsNetworkQualityItem): string {
    const tone = this.qualityTone(item);
    if (tone === 'status-bad') {
      return '差';
    }
    if (tone === 'status-warn') {
      return '一般';
    }
    return '良好';
  }

  formatTime(seconds?: number): string {
    if (!seconds) {
      return '-';
    }
    return new Date(seconds * 1000).toLocaleString();
  }

  formatSampleTime(ms?: number): string {
    if (!ms) {
      return '-';
    }
    return new Date(ms).toLocaleString();
  }
}
