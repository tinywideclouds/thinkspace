import { Injectable, signal } from '@angular/core';

@Injectable({ providedIn: 'root' })
export class UiSettingsService {
  public pagination = signal({ size: 15, prefetchIndex: 4 });
  public tagFilterMode = signal<'visual' | 'context'>('visual');
  public bulkActionBarPosition = signal<'top' | 'bottom'>('top');
}