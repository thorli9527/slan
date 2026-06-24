import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-punch-nodes-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './punch-nodes-page.component.html',
})
// P2P 打洞节点页面，展示 biz 管理的 punch-service 公网 UDP IP、端口和健康状态。
export class PunchNodesPageComponent {
  // 根组件下发的共享视图模型，包含打洞节点列表、表单和启停维护动作。
  @Input({ required: true }) vm!: any;
}
