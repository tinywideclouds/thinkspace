import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';
import { vi } from 'vitest';

(globalThis as any).Temporal = {
  Now: {
    instant: () => ({ toString: () => '2026-10-23T12:00:00Z' })
  },
  Instant: {
    from: (s: string) => ({ toString: () => s })
  }
};

setupTestBed({ zoneless: true });

Element.prototype.scrollTo = vi.fn();

// Mock IntersectionObserver for JSDOM environment without empty functions
Object.defineProperty(global, 'IntersectionObserver', {
  writable: true,
  configurable: true,
  value: class IntersectionObserver {
    readonly root: Element | Document | null = null;
    readonly rootMargin: string = '';
    readonly thresholds: ReadonlyArray<number> = [];
    observe = vi.fn();
    unobserve = vi.fn();
    disconnect = vi.fn();
    takeRecords = vi.fn(() => []);
  }
});