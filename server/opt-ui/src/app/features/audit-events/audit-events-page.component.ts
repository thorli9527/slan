import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { PaginationComponent } from '../../shared/pagination.component';
import { PaginationState } from '../../shared/pagination';

@Component({
  selector: 'ops-audit-events-page',
  standalone: true,
  imports: [CommonModule, FormsModule, PaginationComponent],
  templateUrl: './audit-events-page.component.html',
})
export class AuditEventsPageComponent extends PaginationState {
  @Input({ required: true }) vm!: any;

  get pagedAuditEvents(): any[] {
    return this.pageItems(this.vm.filteredAuditEvents);
  }
}
