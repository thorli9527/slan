import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-network-security-panel',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './network-security-panel.component.html',
  host: {
    class: 'detail-panel-host',
  },
})
export class NetworkSecurityPanelComponent {
  @Input({ required: true }) vm!: any;
}
