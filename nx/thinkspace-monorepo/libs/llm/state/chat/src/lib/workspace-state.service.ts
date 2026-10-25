import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { firstValueFrom } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class WorkspaceStateService {
  private httpClient = inject(HttpClient);
  private thinkspace = inject(ThinkspaceStateService);

  /**
   * Fetches the historical state of a file from the backend disk/git index.
   * @param path The relative file path (e.g., 'src/main.go')
   * @param ref The commit hash, timestamp, or version identifier
   */
  public async getFileContent(path: string, ref: string): Promise<string> {
    const spaceId = this.thinkspace.activeSpaceId();
    if (!spaceId) {
      throw new Error('No active space selected.');
    }

    const params = new HttpParams().set('path', path);
    const url = `/api/spaces/${encodeURIComponent(spaceId)}/files/${encodeURIComponent(ref)}`;

    try {
      return await firstValueFrom(
        this.httpClient.get(url, { params, responseType: 'text' })
      );
    } catch (error) {
      console.error(`[WorkspaceStateService] Failed to load file content for ${path} at ref ${ref}`, error);
      throw error;
    }
  }
}