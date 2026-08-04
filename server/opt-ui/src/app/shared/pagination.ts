export const DEFAULT_PAGE_SIZE = 10;

export function paginate<T>(items: T[], page: number, pageSize: number): T[] {
  const size = Math.max(1, pageSize);
  const pages = Math.max(1, Math.ceil(items.length / size));
  const current = Math.min(Math.max(1, page), pages);
  const start = (current - 1) * size;
  return items.slice(start, start + size);
}

export class PaginationState {
  page = 1;
  pageSize = DEFAULT_PAGE_SIZE;

  pageItems<T>(items: T[]): T[] {
    return paginate(items, this.page, this.pageSize);
  }

  resetPage(): void {
    this.page = 1;
  }
}
