import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-operators-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './operators-page.component.html',
})
export class OperatorsPageComponent {
  @Input({ required: true }) vm!: any;
}
