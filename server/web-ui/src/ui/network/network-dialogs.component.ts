import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-network-dialogs',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './network-dialogs.component.html',
})
export class NetworkDialogsComponent {
  @Input({ required: true }) vm!: any;
}
