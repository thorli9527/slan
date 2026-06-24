import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-relay-nodes-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './relay-nodes-page.component.html',
})
// 中继节点页面，展示 Relay/DERP 节点入口、容量、会话数和维护状态。
export class RelayNodesPageComponent {
  // 根组件下发的共享视图模型，包含中继节点列表、表单和状态管理动作。
  @Input({ required: true }) vm!: any;
}
