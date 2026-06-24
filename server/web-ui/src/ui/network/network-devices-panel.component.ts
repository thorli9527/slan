import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-network-devices-panel',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './network-devices-panel.component.html',
})
export class NetworkDevicesPanelComponent {
  @Input({ required: true }) vm!: any;
}
