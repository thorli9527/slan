import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({selector: 'ops-device-groups-page', standalone: true, imports: [CommonModule], templateUrl: './device-groups-page.component.html'})
export class DeviceGroupsPageComponent { @Input({required: true}) vm!: any; }
