import { TestBed } from '@angular/core/testing';
import { UiSettingsService } from './ui-settings.service';
import { describe, beforeEach, it, expect } from 'vitest';

describe('UiSettingsService', () => {
  let service: UiSettingsService;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [UiSettingsService]
    });
    service = TestBed.inject(UiSettingsService);
  });

  it('should be created with a default history page size of 20 and prefetch index of 4', () => {
    expect(service).toBeTruthy();
    expect(service.pagination().size).toBe(20);
    expect(service.pagination().prefetchIndex).toBe(4);
  });

  it('should allow updating the pagination settings', () => {
    service.pagination.set({ size: 5, prefetchIndex: 2 });
    expect(service.pagination().size).toBe(5);
    expect(service.pagination().prefetchIndex).toBe(2);
  });
});