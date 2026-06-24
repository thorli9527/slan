import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-products-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './products-page.component.html',
})
// 商品管理页面，维护套餐商品、流量包和企业合同包的售卖配置。
export class ProductsPageComponent {
  // 根组件下发的共享视图模型，包含商品列表、套餐引用和保存动作。
  @Input({ required: true }) vm!: any;
}
