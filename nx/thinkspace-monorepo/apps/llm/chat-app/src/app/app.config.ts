import {
  ApplicationConfig,
  provideBrowserGlobalErrorListeners,
} from '@angular/core';
import { provideRouter } from '@angular/router';
import { appRoutes } from './app.routes';

import { TransportService } from '@org/llm-core-transport';
import { MockTransportService } from './mocks/mock-transport.service';
import { WorkspaceStateService } from '@org/llm-state-chat';
import { MockWorkspaceStateService } from './mocks/mock-workspace-state.service';

const useMock = import.meta.env.VITE_USE_MOCK === 'true';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(), 
    provideRouter(appRoutes),
    {
      provide: TransportService,
      useClass: useMock ? MockTransportService : TransportService,
    },
    {
      provide: WorkspaceStateService,
      useClass: useMock ? MockWorkspaceStateService : WorkspaceStateService,
    }
  ],
};
