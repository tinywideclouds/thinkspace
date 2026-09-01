import { inject, Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';
import { SelectionService } from './selection.service';
import { BundleResponse } from '@org/contexter-shared';

@Injectable({ providedIn: 'root' })
export class BundleService {
  private readonly http = inject(HttpClient);
  private readonly selection = inject(SelectionService);
  private readonly baseUrl = 'http://localhost:3333/api';

  readonly generatedBundle = signal<string>('');
  
  readonly currentContextFilename = signal<string | null>(null);
  readonly nextContextFilename = signal<string>('context_001.md');
  readonly nextSelectionFilename = signal<string>('selection_001.yaml');
  readonly existingContexts = signal<string[]>([]);
  readonly existingSelections = signal<string[]>([]);

  async loadDefaults() {
    try {
      const defaults = await firstValueFrom(
        this.http.get<{
          nextContext: string, nextSelection: string,
          existingContexts: string[], existingSelections: string[]
        }>(`${this.baseUrl}/defaults`)
      );
      this.nextContextFilename.set(defaults.nextContext);
      this.nextSelectionFilename.set(defaults.nextSelection);
      this.existingContexts.set(defaults.existingContexts);
      this.existingSelections.set(defaults.existingSelections);
    } catch (e) {
      console.error('[UI] Failed to load defaults:', e);
    }
  }

  async generateBundle() {
    const files = Array.from(this.selection.selectedFiles());
    if (files.length === 0) return '';
    const response = await firstValueFrom(
      this.http.post<BundleResponse>(`${this.baseUrl}/generate`, { files })
    );
    this.generatedBundle.set(response.content);
    return response.content;
  }

  async saveContextToDisk(markdown: string, filename?: string, description?: string) {
    if (!markdown) return null;
    const res = await firstValueFrom(
      this.http.post<{success: boolean, filename: string}>(`${this.baseUrl}/contexts`, { 
        markdown, filename, description
      })
    );
    if (res?.success) {
      this.currentContextFilename.set(res.filename);
    }
    return res;
  }
}