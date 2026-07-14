import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-network-device-groups-panel',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './network-device-groups-panel.component.html',
  host: {
    class: 'detail-panel-host',
  },
})
export class NetworkDeviceGroupsPanelComponent {
  @Input({ required: true }) vm!: any;
}
