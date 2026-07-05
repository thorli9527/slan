import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-network-devices-panel',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './network-devices-panel.component.html',
  host: {
    class: 'detail-panel-host',
  },
})
export class NetworkDevicesPanelComponent {
  @Input({ required: true }) vm!: any;
}
