import { AppComponentNetworks } from '../network/app.component.networks';
import { ApiUserAlias, UserAliasRow } from '../app.models';
import { WEB_API } from '../api-paths';
import { DEFAULT_USER_ID } from '../app.seed-data';

export abstract class AppComponentUserAlias extends AppComponentNetworks {
  openUserAliasDialog(user: UserAliasRow): void {
    if (this.showUserAliasDialog && this.editingUserAlias?.email === user.email) {
      this.closeUserAliasDialog();
      return;
    }
    this.editingUserAlias = user;
    this.userAliasValue = user.alias || user.email.split('@')[0] || '';
    this.showUserAliasDialog = true;
  }

  closeUserAliasDialog(): void {
    this.showUserAliasDialog = false;
    this.editingUserAlias = null;
  }

  async saveUserAliasDialog(): Promise<void> {
    if (!this.editingUserAlias || !this.userAliasValue.trim()) {
      return;
    }
    const email = this.editingUserAlias.email;
    const alias = this.userAliasValue.trim();
    try {
      const updated = await this.api.patch<ApiUserAlias>(WEB_API.userAliases, {
        ownerUserId: this.currentUserId || DEFAULT_USER_ID,
        email,
        alias,
      });
      this.upsertUserAlias({ email: updated.email, alias: updated.alias });
      this.closeUserAliasDialog();
      return;
    } catch {
      // Local preview mode updates below.
    }
    this.upsertUserAlias({ email, alias });
    this.closeUserAliasDialog();
  }

  private upsertUserAlias(aliasRow: UserAliasRow): void {
    const email = aliasRow.email;
    const existing = this.userAliases.find((item) => item.email === email);
    if (existing) {
      existing.alias = aliasRow.alias;
    } else {
      this.userAliases = [...this.userAliases, aliasRow];
    }
  }
}
