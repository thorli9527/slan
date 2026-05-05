import { Injectable, inject } from '@angular/core';
import { DevicesOpsService } from '../features/devices/devices-ops.service';
import { IceOpsService } from '../features/ice/ice-ops.service';
import { OverviewOpsService } from '../features/overview/overview-ops.service';
import { QualityOpsService } from '../features/quality/quality-ops.service';
import { RbacOpsService } from '../features/rbac/rbac-ops.service';
import { RelayOpsService } from '../features/relay/relay-ops.service';
import { SettingsOpsService } from '../features/settings/settings-ops.service';
import { UsersOpsService } from '../features/users/users-ops.service';
import { WireOpsService } from '../features/wire/wire-ops.service';
import { AuthOpsService } from './auth-ops.service';
import { IceServerDraft, OpsIceServer, PlanConfig, WireNodeEventFilter } from './models';

@Injectable({ providedIn: 'root' })
export class OpsStoreService {
  readonly auth = inject(AuthOpsService);
  readonly devices = inject(DevicesOpsService);
  readonly ice = inject(IceOpsService);
  readonly overview = inject(OverviewOpsService);
  readonly quality = inject(QualityOpsService);
  readonly rbac = inject(RbacOpsService);
  readonly relay = inject(RelayOpsService);
  readonly settings = inject(SettingsOpsService);
  readonly users = inject(UsersOpsService);
  readonly wire = inject(WireOpsService);

  login(loginName: string, password: string) {
    return this.auth.login(loginName, password);
  }

  logout(token: string) {
    return this.auth.logout(token);
  }

  changeOwnPassword(token: string, password: string) {
    return this.auth.changeOwnPassword(token, password);
  }

  wireEvents(token: string, filter: WireNodeEventFilter) {
    return this.wire.fetchEvents(token, filter);
  }

  saveIceServer(token: string, draft: IceServerDraft, editingServerId: string) {
    return this.ice.saveServer(token, draft, editingServerId);
  }

  updateIceStatus(token: string, server: OpsIceServer, status: string) {
    return this.ice.updateStatus(token, server.serverId, status);
  }

  saveUserPlanOverride(token: string, userId: string, plan: PlanConfig) {
    return this.users.savePlanOverride(token, userId, plan);
  }

  clearUserPlanOverride(token: string, userId: string) {
    return this.users.clearPlanOverride(token, userId);
  }

  savePlanConfig(token: string, plan: PlanConfig) {
    return this.settings.savePlanConfig(token, plan);
  }
}
