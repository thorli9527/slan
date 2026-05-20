import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-customers-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './customers-page.component.html',
})
export class CustomersPageComponent {
  @Input({ required: true }) vm!: any;
}
