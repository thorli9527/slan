import { CommonModule } from '@angular/common';
import { Component, input, output } from '@angular/core';
import { OpsUser } from '../../shared/models';

@Component({
  selector: 'slan-users-view',
  standalone: true,
  imports: [CommonModule],
  template: `
    <section class="panel">
      <table>
        <thead><tr><th>邮箱</th><th>用户 ID</th><th>设备</th><th>节点</th><th>角色</th><th>专属配置</th><th>操作</th></tr></thead>
        <tbody>
          <tr *ngFor="let user of users()">
            <td>{{ user.email }}</td>
            <td><code>{{ user.userId }}</code></td>
            <td>{{ user.deviceCount }}</td>
            <td>{{ user.nodeCount }}</td>
            <td>{{ user.roleCodes?.join(', ') || '-' }}</td>
            <td>{{ user.planOverride ? '已配置' : '继承全局' }}</td>
            <td><button type="button" (click)="configure.emit(user)">配置</button></td>
          </tr>
        </tbody>
      </table>
      <div class="pager" *ngIf="filteredCount() > pageSize()">
        <span>{{ pageSummary() }}</span>
        <button type="button" (click)="previousPage.emit()" [disabled]="!canPrevious()">上一页</button>
        <button type="button" (click)="nextPage.emit()" [disabled]="!canNext()">下一页</button>
      </div>
    </section>
  `,
})
export class UsersViewComponent {
  readonly users = input.required<OpsUser[]>();
  readonly filteredCount = input(0);
  readonly pageSize = input(10);
  readonly pageSummary = input('');
  readonly canPrevious = input(false);
  readonly canNext = input(false);
  readonly configure = output<OpsUser>();
  readonly previousPage = output<void>();
  readonly nextPage = output<void>();
}
