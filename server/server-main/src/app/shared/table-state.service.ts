import { Injectable, signal } from '@angular/core';
import { PagedViewKey, ViewKey } from './models';

@Injectable({ providedIn: 'root' })
export class TableStateService {
  readonly keyword = signal('');
  readonly pageSize = 10;
  readonly pages = signal<Record<PagedViewKey, number>>({
    users: 1,
    devices: 1,
    admins: 1,
    roles: 1,
    menus: 1,
    wire: 1,
    ice: 1,
    quality: 1,
  });

  setKeyword(value: string, view: ViewKey): void {
    this.keyword.set(value);
    if (this.isPagedView(view)) {
      this.setPage(view, 1);
    }
  }

  pageSummary(view: PagedViewKey, total: number): string {
    if (!total) {
      return '0 / 0';
    }
    const page = this.currentPage(view, total);
    const start = (page - 1) * this.pageSize + 1;
    const end = Math.min(page * this.pageSize, total);
    return `${start}-${end} / ${total}`;
  }

  canPrevious(view: PagedViewKey): boolean {
    return (this.pages()[view] || 1) > 1;
  }

  canNext(view: PagedViewKey, total: number): boolean {
    return (this.pages()[view] || 1) < this.totalPages(total);
  }

  previous(view: PagedViewKey): void {
    this.setPage(view, Math.max(1, (this.pages()[view] || 1) - 1));
  }

  next(view: PagedViewKey, total: number): void {
    this.setPage(view, Math.min(this.totalPages(total), (this.pages()[view] || 1) + 1));
  }

  rows<T>(view: PagedViewKey, rows: T[]): T[] {
    const page = this.currentPage(view, rows.length);
    const start = (page - 1) * this.pageSize;
    return rows.slice(start, start + this.pageSize);
  }

  resetView(view: ViewKey): void {
    if (this.isPagedView(view)) {
      this.setPage(view, 1);
    }
  }

  isPagedView(view: ViewKey): view is PagedViewKey {
    return view !== 'overview' && view !== 'relays' && view !== 'settings';
  }

  private currentPage(view: PagedViewKey, total: number): number {
    return Math.min(Math.max(1, this.pages()[view] || 1), this.totalPages(total));
  }

  private totalPages(total: number): number {
    return Math.max(1, Math.ceil(total / this.pageSize));
  }

  private setPage(view: PagedViewKey, page: number): void {
    this.pages.update((pages) => ({ ...pages, [view]: page }));
  }
}

export function filterRows<T>(rows: T[], keyword: string): T[] {
  const value = keyword.trim().toLowerCase();
  if (!value) {
    return rows;
  }
  return rows.filter((row) => JSON.stringify(row).toLowerCase().includes(value));
}
