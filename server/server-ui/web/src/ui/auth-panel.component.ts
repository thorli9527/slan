import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, Output } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { AuthMode } from './ui-models';

@Component({
  selector: 'slan-auth-panel',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './auth-panel.component.html',
  styleUrl: './auth-panel.component.css'
})
export class AuthPanelComponent {
  @Input({ required: true }) mode!: AuthMode;
  @Input({ required: true }) email!: string;
  @Input({ required: true }) password!: string;

  @Output() readonly modeChange = new EventEmitter<AuthMode>();
  @Output() readonly emailChange = new EventEmitter<string>();
  @Output() readonly passwordChange = new EventEmitter<string>();
  @Output() readonly submit = new EventEmitter<void>();
}
