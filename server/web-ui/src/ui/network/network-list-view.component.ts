import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-network-list-view',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './network-list-view.component.html',
})
export class NetworkListViewComponent {
  @Input({ required: true }) vm!: any;
}
