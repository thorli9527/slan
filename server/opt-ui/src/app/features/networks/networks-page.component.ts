import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { PaginationComponent } from '../../shared/pagination.component';
import { PaginationState } from '../../shared/pagination';

@Component({ selector: 'ops-networks-page', standalone: true, imports: [CommonModule, FormsModule, PaginationComponent], templateUrl: './networks-page.component.html' })
export class NetworksPageComponent extends PaginationState {
  @Input({ required: true }) vm!: any;
  get pagedNetworks(): any[] { return this.pageItems(this.vm.networks); }
}
