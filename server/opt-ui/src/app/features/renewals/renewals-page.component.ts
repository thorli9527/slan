import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-renewals-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './renewals-page.component.html',
})
// 续费管理页面，记录客户套餐有效期延长和人工/支付来源。
export class RenewalsPageComponent {
  // 根组件下发的共享视图模型，包含续费列表、金额和有效期表单。
  @Input({ required: true }) vm!: any;
}
