import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-products-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './products-page.component.html',
})
export class ProductsPageComponent {
  @Input({ required: true }) vm!: any;
}
