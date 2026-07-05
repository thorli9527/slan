import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-network-dns-panel',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './network-dns-panel.component.html',
  host: {
    class: 'detail-panel-host',
  },
})
export class NetworkDnsPanelComponent {
  @Input({ required: true }) vm!: any;
}
