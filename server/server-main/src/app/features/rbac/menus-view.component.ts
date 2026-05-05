import { CommonModule } from '@angular/common';
import { Component, input, output } from '@angular/core';
import { OpsMenu } from '../../shared/models';

@Component({
  selector: 'slan-menus-view',
  standalone: true,
  imports: [CommonModule],
  template: `
    <section class="panel">
      <table>
        <thead><tr><th>编码</th><th>名称</th><th>路径</th><th>排序</th><th>状态</th></tr></thead>
        <tbody>
          <tr *ngFor="let menu of menus()">
            <td><code>{{ menu.menuCode }}</code></td>
            <td>{{ menu.menuName }}</td>
            <td>{{ menu.path || '-' }}</td>
            <td>{{ menu.sort }}</td>
            <td><span class="pill">{{ menu.status }}</span></td>
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
export class MenusViewComponent {
  readonly menus = input.required<OpsMenu[]>();
  readonly totalCount = input(0);
  readonly pageSize = input(10);
  readonly pageSummary = input('');
  readonly canPrevious = input(false);
  readonly canNext = input(false);
  readonly previousPage = output<void>();
  readonly nextPage = output<void>();
}
