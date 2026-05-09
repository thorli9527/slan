import { CommonModule } from '@angular/common';
import { Component, computed, input, output } from '@angular/core';
import { OpsRelayTopology } from '../../shared/models';

@Component({
  selector: 'slan-relay-view',
  standalone: true,
  imports: [CommonModule],
  template: `
    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Relay 节点管理</h2>
          <p class="muted">默认集群：{{ relays()?.defaultClusterId || '-' }}</p>
        </div>
        <button type="button" (click)="refresh.emit()" [disabled]="loading()">刷新</button>
      </div>
      <div class="relay-summary">
        <article class="metric compact">
          <span>节点在线</span>
          <strong>{{ relayOnlineCount() }}/{{ relayNodes().length }}</strong>
        </article>
        <article class="metric compact">
          <span>区域</span>
          <strong>{{ relayRegionCount() }}</strong>
        </article>
        <article class="metric compact">
          <span>活跃会话</span>
          <strong>{{ relayActiveSessions() }}</strong>
        </article>
      </div>
      <table>
        <thead><tr><th>节点</th><th>集群</th><th>区域</th><th>地址</th><th>心跳</th><th>会话</th><th>质量</th><th>优先级</th></tr></thead>
        <tbody>
          <tr *ngFor="let node of relayNodes()">
            <td><code>{{ node.nodeId }}</code></td>
            <td>{{ node.clusterName }}</td>
            <td>{{ node.countryName || '-' }} {{ node.cityName || '' }}</td>
            <td>{{ node.transport }} · {{ node.address }}</td>
            <td>
              <span [class.status-ok]="node.heartbeatOnline" [class.status-bad]="!node.heartbeatOnline">
                {{ node.heartbeatOnline ? '在线' : '不在线' }}
              </span>
              <small>{{ formatTime(node.heartbeatLastSeenAt) }}</small>
            </td>
            <td>{{ node.activeSessions || 0 }}</td>
            <td>
              <span>RTT {{ node.observedRttMs || '-' }}</span>
              <small>丢包 {{ node.packetLossPpm ?? '-' }} · 评分 {{ node.pathScore ?? '-' }}</small>
            </td>
            <td>{{ node.priority }}</td>
          </tr>
        </tbody>
      </table>
      <section class="relay-regions" *ngIf="relays()?.regions?.length">
        <h3>区域与候选入口</h3>
        <div class="region-grid">
          <article class="region-card" *ngFor="let region of relays()?.regions || []">
            <strong>{{ region.countryName || '-' }} {{ region.cityName || '' }}</strong>
            <span>{{ region.clusterName || region.clusterId || '-' }}</span>
            <small *ngFor="let endpoint of region.endpoints || []">{{ endpoint.transport }} · {{ endpoint.address }}</small>
          </article>
        </div>
      </section>
    </section>
  `,
})
export class RelayViewComponent {
  readonly relays = input<OpsRelayTopology | null>(null);
  readonly loading = input(false);
  readonly refresh = output<void>();

  readonly relayNodes = computed(() => this.relays()?.nodes || []);
  readonly relayOnlineCount = computed(() => this.relayNodes().filter((node) => node.heartbeatOnline).length);
  readonly relayRegionCount = computed(() => this.relays()?.regions?.length || 0);
  readonly relayActiveSessions = computed(() => this.relayNodes().reduce((total, node) => total + (node.activeSessions || 0), 0));

  formatTime(seconds?: number): string {
    if (!seconds) {
      return '-';
    }
    return new Date(seconds * 1000).toLocaleString();
  }
}
