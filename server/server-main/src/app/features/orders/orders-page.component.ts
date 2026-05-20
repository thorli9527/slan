import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-orders-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './orders-page.component.html',
})
export class OrdersPageComponent {
  @Input({ required: true }) vm!: any;
}
