import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'ops-client-downloads-page',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './client-downloads-page.component.html',
})
export class ClientDownloadsPageComponent {
  @Input({ required: true }) vm!: any;
}
