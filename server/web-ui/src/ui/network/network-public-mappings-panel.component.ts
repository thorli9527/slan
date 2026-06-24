import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'app-network-public-mappings-panel',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './network-public-mappings-panel.component.html',
})
export class NetworkPublicMappingsPanelComponent {
  @Input({ required: true }) vm!: any;
}
