import { CommonModule } from '@angular/common';
import { Component, ViewEncapsulation } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { AppApiClient } from './app-api.service';
import { AppComponentAuth } from './app.component.auth';
import { DevicePageComponent } from './device/device-page.component';
import { NetworkPageComponent } from './network/network-page.component';
import { OverviewPageComponent } from './overview/overview-page.component';
import { UserAliasPageComponent } from './user-alias/user-alias-page.component';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [CommonModule, FormsModule, OverviewPageComponent, DevicePageComponent, UserAliasPageComponent, NetworkPageComponent],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css',
  encapsulation: ViewEncapsulation.None,
})
export class AppComponent extends AppComponentAuth {
  constructor(api: AppApiClient) {
    super(api);
  }

  ngOnInit(): void {
    this.applyRouteFromLocation();
    window.addEventListener('popstate', () => this.applyRouteFromLocation());
  }

  get vm(): this {
    return this;
  }
}
