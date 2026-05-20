import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'ops-devices-page',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './devices-page.component.html',
})
export class DevicesPageComponent {
  @Input({ required: true }) vm!: any;
}
