import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-overview-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './overview-page.component.html',
})
export class OverviewPageComponent {
  @Input({ required: true }) vm!: any;
}
