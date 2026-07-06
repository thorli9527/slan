import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'app-network-device-groups-panel',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './network-device-groups-panel.component.html',
  host: {
    class: 'detail-panel-host',
  },
})
export class NetworkDeviceGroupsPanelComponent {
  @Input({ required: true }) vm!: any;
}
