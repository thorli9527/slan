import { Injectable, inject, signal } from '@angular/core';
import { OpsOverview } from '../../shared/models';
import { OpsApiService } from '../../shared/ops-api.service';

@Injectable({ providedIn: 'root' })
export class OverviewOpsService {
  private readonly api = inject(OpsApiService);
  readonly overview = signal<OpsOverview | null>(null);

  async refresh(token: string): Promise<void> {
    this.overview.set(await this.fetchOverview(token));
  }

  fetchOverview(token: string): Promise<OpsOverview> {
    return this.api.request<OpsOverview>('/overview', token);
  }
}
