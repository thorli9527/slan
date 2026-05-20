import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-device-page',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './device-page.component.html',
})
export class DevicePageComponent {
  @Input({ required: true }) vm!: any;
}
