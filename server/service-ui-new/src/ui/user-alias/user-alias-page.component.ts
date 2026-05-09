import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-user-alias-page',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './user-alias-page.component.html',
})
export class UserAliasPageComponent {
  @Input({ required: true }) vm!: any;
}
