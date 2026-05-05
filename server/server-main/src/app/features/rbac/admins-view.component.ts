import { CommonModule } from '@angular/common';
import { Component, input, output } from '@angular/core';
import { OpsAdmin } from '../../shared/models';

@Component({
  selector: 'slan-admins-view',
  standalone: true,
  imports: [CommonModule],
  template: `
    <section class="panel">
      <table>
        <thead><tr><th>登录名</th><th>显示名</th><th>用户</th><th>部门</th><th>状态</th><th>角色</th></tr></thead>
        <tbody>
          <tr *ngFor="let admin of admins()">
            <td>{{ admin.loginName }}</td>
            <td>{{ admin.displayName }}</td>
            <td><code>{{ admin.userId }}</code></td>
            <td>{{ admin.department || '-' }}</td>
            <td><span class="pill">{{ admin.status }}</span></td>
            <td>{{ admin.roleCodes?.join(', ') || '-' }}</td>
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
export class AdminsViewComponent {
  readonly admins = input.required<OpsAdmin[]>();
  readonly filteredCount = input(0);
  readonly pageSize = input(10);
  readonly pageSummary = input('');
  readonly canPrevious = input(false);
  readonly canNext = input(false);
  readonly previousPage = output<void>();
  readonly nextPage = output<void>();
}
