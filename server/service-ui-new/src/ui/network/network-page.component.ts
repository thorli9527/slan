import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-network-page',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './network-page.component.html',
})
export class NetworkPageComponent {
  @Input({ required: true }) vm!: any;
}
