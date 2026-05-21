import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'ops-devices-page',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './devices-page.component.html',
})
// 设备管理页面，展示全局设备地址、在线状态、启用状态和流量统计。
export class DevicesPageComponent {
  // 根组件下发的共享视图模型，包含筛选关键字、设备列表和状态切换动作。
  @Input({ required: true }) vm!: any;
}
