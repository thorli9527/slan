import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-renewals-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './renewals-page.component.html',
})
export class RenewalsPageComponent {
  @Input({ required: true }) vm!: any;
}
