import { CommonModule } from '@angular/common';
import { Component, computed, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { WireNodeBase, WireNodeEventFilter, WireNodeEventList, WireNodesOpsView } from '../../shared/models';

@Component({
  selector: 'slan-wire-view',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './wire-view.component.html',
})
export class WireViewComponent {
  readonly wireNodes = input<WireNodesOpsView | null>(null);
  readonly wireEvents = input.required<WireNodeEventList>();
  readonly filter = input.required<WireNodeEventFilter>();
  readonly loading = input(false);
  readonly refresh = output<void>();
  readonly applyFilter = output<void>();
  readonly resetFilter = output<void>();
  readonly previousPage = output<void>();
  readonly nextPage = output<void>();

  readonly wireDerpNodes = computed(() => this.wireNodes()?.derpNodes || []);
  readonly wireRelayNodes = computed(() => this.wireNodes()?.relayNodes || []);
  readonly wireNodeTotalCount = computed(() => this.wireDerpNodes().length + this.wireRelayNodes().length);

  wireNodeStatus(node: WireNodeBase): string {
    if (!node.enabled) {
      return 'disabled';
    }
    if (node.stale) {
      return 'stale';
    }
    if (!node.healthy) {
      return 'unhealthy';
    }
    return 'ok';
  }

  wireNodeStatusClass(node: WireNodeBase): string {
    return this.wireNodeStatus(node) === 'ok' ? 'status-ok' : 'status-bad';
  }

  boolTransition(from?: boolean, to?: boolean): string {
    if (from === undefined && to === undefined) {
      return '-';
    }
    if (from === undefined) {
      return `->${to}`;
    }
    if (to === undefined) {
      return `${from}->`;
    }
    return `${from}->${to}`;
  }

  formatAgeMs(ms?: number): string {
    if (!ms) {
      return '-';
    }
    const seconds = Math.max(0, Math.floor((Date.now() - ms) / 1000));
    if (seconds < 60) {
      return `${seconds}s`;
    }
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) {
      return `${minutes}m`;
    }
    return `${Math.floor(minutes / 60)}h`;
  }

  formatTimeMs(ms?: number): string {
    if (!ms) {
      return '-';
    }
    return new Date(ms).toLocaleString();
  }

  canNextWireEventPage(): boolean {
    return this.filter().page * this.filter().pageSize < this.wireEvents().total;
  }
}
