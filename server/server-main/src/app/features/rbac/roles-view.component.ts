import { CommonModule } from '@angular/common';
import { Component, input, output } from '@angular/core';
import { OpsRole } from '../../shared/models';

@Component({
  selector: 'slan-roles-view',
  standalone: true,
  imports: [CommonModule],
  template: `
    <section class="panel">
      <table>
        <thead><tr><th>编码</th><th>名称</th><th>内置</th><th>菜单</th></tr></thead>
        <tbody>
          <tr *ngFor="let role of roles()">
            <td><code>{{ role.roleCode }}</code></td>
            <td>{{ role.roleName }}</td>
            <td>{{ role.builtin ? '是' : '否' }}</td>
            <td>{{ role.menuCodes?.join(', ') || '-' }}</td>
          </tr>
        </tbody>
      </table>
      <div class="pager" *ngIf="totalCount() > pageSize()">
        <span>{{ pageSummary() }}</span>
        <button type="button" (click)="previousPage.emit()" [disabled]="!canPrevious()">上一页</button>
        <button type="button" (click)="nextPage.emit()" [disabled]="!canNext()">下一页</button>
      </div>
    </section>
  `,
})
export class RolesViewComponent {
  readonly roles = input.required<OpsRole[]>();
  readonly totalCount = input(0);
  readonly pageSize = input(10);
  readonly pageSummary = input('');
  readonly canPrevious = input(false);
  readonly canNext = input(false);
  readonly previousPage = output<void>();
  readonly nextPage = output<void>();
}
