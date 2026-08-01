import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-users-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './users-page.component.html',
})
// 用户管理页面，展示应用用户资料、设备数量和账户状态。
export class UsersPageComponent {
  @Input({ required: true }) vm!: any;
}
