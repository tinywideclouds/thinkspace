import { Injectable, signal } from '@angular/core';

@Injectable({ providedIn: 'root' })
export class UiSettingsService {
  public pagination = signal({
    size: 20,
    prefetchIndex: 4
  });
}