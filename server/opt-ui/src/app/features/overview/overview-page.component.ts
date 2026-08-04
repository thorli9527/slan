import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-overview-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './overview-page.component.html',
})
// 运营概览页面，汇总客户、设备、中继和打洞节点关键指标。
export class OverviewPageComponent {
  // 根组件下发的共享视图模型，提供概览统计 getter 和最近数据。
  @Input({ required: true }) vm!: any;
}
