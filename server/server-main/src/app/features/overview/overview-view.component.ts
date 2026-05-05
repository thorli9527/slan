import { CommonModule } from '@angular/common';
import { Component, input } from '@angular/core';
import { OpsOverview } from '../../shared/models';

@Component({
  selector: 'slan-overview-view',
  standalone: true,
  imports: [CommonModule],
  template: `
    <section class="dashboard">
      <article class="metric">
        <span>用户</span>
        <strong>{{ overview()?.userCount || 0 }}</strong>
      </article>
      <article class="metric">
        <span>设备</span>
        <strong>{{ overview()?.deviceCount || 0 }}</strong>
        <small>{{ overview()?.onlineDeviceCount || 0 }} 在线</small>
      </article>
      <article class="metric">
        <span>节点</span>
        <strong>{{ overview()?.nodeCount || 0 }}</strong>
      </article>
      <article class="metric">
        <span>Relay</span>
        <strong>{{ overview()?.relayNodeCount || 0 }}</strong>
        <small>{{ overview()?.relayOnlineNodeCount || 0 }} 在线 · {{ overview()?.relayClusterCount || 0 }} 集群</small>
      </article>
      <article class="panel span-2">
        <h2>安全状态</h2>
        <p>默认管理员：{{ overview()?.defaultAdminSeeded ? '已创建' : '未创建' }}</p>
        <p>默认角色绑定：{{ overview()?.defaultAdminRoleBound ? '正常' : '缺失' }}</p>
        <ul *ngIf="overview()?.securityWarnings?.length">
          <li *ngFor="let item of overview()?.securityWarnings">{{ item }}</li>
        </ul>
      </article>
    </section>
  `,
})
export class OverviewViewComponent {
  readonly overview = input<OpsOverview | null>(null);
}
