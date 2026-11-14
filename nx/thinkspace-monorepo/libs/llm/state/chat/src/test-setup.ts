import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed({ zoneless: true });

(globalThis as any).Temporal = {
  Now: {
    instant: () => ({ toString: () => '2026-10-23T12:00:00Z' })
  },
  Instant: {
    from: (s: string) => ({ toString: () => s })
  }
};
