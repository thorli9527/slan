import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { PaginationComponent } from '../../shared/pagination.component';
import { PaginationState } from '../../shared/pagination';

@Component({
  selector: 'ops-customers-page',
  standalone: true,
  imports: [CommonModule, PaginationComponent],
  templateUrl: './customers-page.component.html',
})
// 客户管理页面，展示客户地域、Relay 用量和启用状态。
export class CustomersPageComponent extends PaginationState {
  // 根组件下发的共享视图模型，包含客户列表和编辑动作。
  @Input({ required: true }) vm!: any;

  get pagedCustomers(): any[] {
    return this.pageItems(this.vm.customers);
  }
}
