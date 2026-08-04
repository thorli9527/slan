import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { PaginationComponent } from '../../shared/pagination.component';
import { PaginationState } from '../../shared/pagination';

@Component({ selector: 'ops-device-groups-page', standalone: true, imports: [CommonModule, FormsModule, PaginationComponent], templateUrl: './device-groups-page.component.html' })
export class DeviceGroupsPageComponent extends PaginationState {
  @Input({ required: true }) vm!: any;
  get pagedGroups(): any[] { return this.pageItems(this.vm.deviceGroups); }
}
