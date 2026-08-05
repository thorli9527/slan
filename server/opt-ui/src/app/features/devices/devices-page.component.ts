import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { PaginationComponent } from '../../shared/pagination.component';
import { PaginationState } from '../../shared/pagination';

@Component({
  selector: 'ops-devices-page',
  standalone: true,
  imports: [CommonModule, FormsModule, PaginationComponent],
  templateUrl: './devices-page.component.html',
})
// 设备管理页面，展示全局设备地址、在线状态、启用状态和创建时间。
export class DevicesPageComponent extends PaginationState {
  // 根组件下发的共享视图模型，包含筛选关键字、设备列表和状态切换动作。
  @Input({ required: true }) vm!: any;

  get pagedDevices(): any[] {
    return this.pageItems(this.vm.filteredDevices);
  }
}
