import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-customers-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './customers-page.component.html',
})
// 客户管理页面，展示客户套餐、地域、设备数量和启用/限流状态。
export class CustomersPageComponent {
  // 根组件下发的共享视图模型，包含客户列表、套餐分配和编辑动作。
  @Input({ required: true }) vm!: any;
}
