import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { NetworkDetailViewComponent } from './network-detail-view.component';
import { NetworkDialogsComponent } from './network-dialogs.component';
import { NetworkListViewComponent } from './network-list-view.component';

@Component({
  selector: 'app-network-page',
  standalone: true,
  imports: [CommonModule, NetworkListViewComponent, NetworkDetailViewComponent, NetworkDialogsComponent],
  templateUrl: './network-page.component.html',
})
export class NetworkPageComponent {
  @Input({ required: true }) vm!: any;
}
