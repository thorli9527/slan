import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Component, Input } from '@angular/core';

@Component({selector: 'ops-networks-page', standalone: true, imports: [CommonModule, FormsModule], templateUrl: './networks-page.component.html'})
export class NetworksPageComponent { @Input({required: true}) vm!: any; }
