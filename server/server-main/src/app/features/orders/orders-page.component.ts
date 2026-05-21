import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-orders-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './orders-page.component.html',
})
// 订单管理页面，展示支付状态、开通状态和订单对应的商品权益。
export class OrdersPageComponent {
  // 根组件下发的共享视图模型，包含订单列表、表单和人工处理动作。
  @Input({ required: true }) vm!: any;
}
