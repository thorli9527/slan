import { CommonModule } from '@angular/common';
import { Component, computed, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { IceServerDraft, OpsIceServer, OpsIceServerStats } from '../../shared/models';

@Component({
  selector: 'slan-ice-view',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './ice-view.component.html',
})
export class IceViewComponent {
  readonly servers = input.required<OpsIceServer[]>();
  readonly stats = input.required<OpsIceServerStats[]>();
  readonly pagedServers = input.required<OpsIceServer[]>();
  readonly filteredCount = input(0);
  readonly pageSize = input(10);
  readonly pageSummary = input('');
  readonly canPrevious = input(false);
  readonly canNext = input(false);
  readonly draft = input.required<IceServerDraft>();
  readonly editingServerId = input('');
  readonly loading = input(false);
  readonly editorOpen = signal(false);

  readonly refresh = output<void>();
  readonly previousPage = output<void>();
  readonly nextPage = output<void>();
  readonly edit = output<OpsIceServer>();
  readonly save = output<void>();
  readonly resetDraft = output<void>();
  readonly updateStatus = output<{ server: OpsIceServer; status: string }>();

  readonly enabledCount = computed(() => this.servers().filter((server) => (server.status || 'enabled') === 'enabled').length);
  readonly candidateCount = computed(() => this.stats().reduce((total, item) => total + item.candidateCount, 0));
  readonly p2pSuccessCount = computed(() => this.stats().reduce((total, item) => total + item.p2pSuccessCount, 0));
  readonly statsByServer = computed(() => {
    const out: Record<string, OpsIceServerStats> = {};
    for (const item of this.stats()) {
      out[item.serverId] = item;
    }
    return out;
  });

  stat(serverId: string): OpsIceServerStats | undefined {
    return this.statsByServer()[serverId];
  }

  openCreate(): void {
    this.resetDraft.emit();
    this.editorOpen.set(true);
  }

  openEdit(server: OpsIceServer): void {
    this.edit.emit(server);
    this.editorOpen.set(true);
  }

  closeEditor(): void {
    this.editorOpen.set(false);
  }
}
