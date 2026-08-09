import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { PaginationComponent } from '../../shared/pagination.component';
import { PaginationState } from '../../shared/pagination';

@Component({
  selector: 'ops-server-nodes-page',
  standalone: true,
  imports: [CommonModule, PaginationComponent],
  templateUrl: './server-nodes-page.component.html',
})
export class ServerNodesPageComponent extends PaginationState {
  @Input({ required: true }) vm!: any;

  get pagedServerNodes(): any[] {
    return this.pageItems(this.vm.serverNodes);
  }
}
