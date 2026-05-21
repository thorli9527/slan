import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-operators-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './operators-page.component.html',
})
// 运营用户页面，维护后台账号、角色、状态和密码重置入口。
export class OperatorsPageComponent {
  // 根组件下发的共享视图模型，包含操作员列表、表单和账号管理方法。
  @Input({ required: true }) vm!: any;
}
