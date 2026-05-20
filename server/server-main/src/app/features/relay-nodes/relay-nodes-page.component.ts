import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';

@Component({
  selector: 'ops-relay-nodes-page',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './relay-nodes-page.component.html',
})
export class RelayNodesPageComponent {
  @Input({ required: true }) vm!: any;
}
