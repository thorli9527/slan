import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { NetworkDeviceGroupsPanelComponent } from './network-device-groups-panel.component';
import { NetworkDevicesPanelComponent } from './network-devices-panel.component';
import { NetworkDnsPanelComponent } from './network-dns-panel.component';
import { NetworkPublicMappingsPanelComponent } from './network-public-mappings-panel.component';
import { NetworkSecurityPanelComponent } from './network-security-panel.component';

@Component({
  selector: 'app-network-detail-view',
  standalone: true,
  imports: [
    CommonModule,
    NetworkDeviceGroupsPanelComponent,
    NetworkDevicesPanelComponent,
    NetworkDnsPanelComponent,
    NetworkPublicMappingsPanelComponent,
    NetworkSecurityPanelComponent,
  ],
  templateUrl: './network-detail-view.component.html',
  host: {
    class: 'network-view-host',
  },
})
export class NetworkDetailViewComponent {
  @Input({ required: true }) vm!: any;
}
