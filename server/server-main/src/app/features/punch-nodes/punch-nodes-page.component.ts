import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-punch-nodes-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './punch-nodes-page.component.html',
})
export class PunchNodesPageComponent {
  @Input({ required: true }) vm!: any;
}
