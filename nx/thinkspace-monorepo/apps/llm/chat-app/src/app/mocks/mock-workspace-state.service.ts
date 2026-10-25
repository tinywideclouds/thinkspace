import { Injectable } from '@angular/core';
import mockGraph from './mock-graph.json';

@Injectable({ providedIn: 'root' })
export class MockWorkspaceStateService {
  public async getFileContent(path: string, ref: string): Promise<string> {
    // Simulate network latency
    await new Promise(resolve => setTimeout(resolve, 600));

    const files = (mockGraph as any).files || {};
    const content = files[ref];

    if (content !== undefined) {
      return content;
    }

    throw new Error(`[Mock] File ref "${ref}" not found in mock-graph.json 'files' directory.`);
  }
}