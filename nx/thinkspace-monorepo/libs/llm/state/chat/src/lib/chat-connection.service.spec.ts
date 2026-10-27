import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { ChatConnectionService } from './chat-connection.service';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { TimelineStore } from './timeline.store';
import { UiSettingsService } from './ui-settings.service';
import { TransportService } from '@org/llm-core-transport';
import { WSEvent } from '@org/llm-core-protos';
import { Subject } from 'rxjs';
import { signal } from '@angular/core';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';

describe('ChatConnectionService (Dispatcher)', () => {
  let service: ChatConnectionService;
  let mockTransportService: { connect: Mock, send: Mock, disconnect: Mock };
  let mockWorkspaceStateService: any;
  let mockTimelineStore: any;
  let mockUiSettingsService: any;
  let webSocketSubject: Subject<WSEvent>;

  beforeEach(() => {
    webSocketSubject = new Subject<WSEvent>();
    
    mockTransportService = {
      connect: vi.fn(() => webSocketSubject.asObservable()),
      send: vi.fn(),
      disconnect: vi.fn(),
    };

    mockWorkspaceStateService = {
      activeSpaceId: signal('golang'),
      activeChatId: signal('test-chat')
    };

    mockTimelineStore = {
      appendCoreChat: vi.fn(),
      appendToLastModelMessage: vi.fn(),
      clearSession: vi.fn(),
      ingestEvent: vi.fn(),
      hasMoreHistory: signal(true),
      isLoadingHistory: signal(false),
      nextCursor: signal('cursor-123'),
      processSyncHistory: vi.fn(),
      coreChat: signal([])
    };

    mockUiSettingsService = {
      pagination: signal({ size: 15, prefetchIndex: 4 })
    };

    TestBed.configureTestingModule({
      providers: [
        ChatConnectionService,
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: TransportService, useValue: mockTransportService },
        { provide: ThinkspaceStateService, useValue: mockWorkspaceStateService },
        { provide: TimelineStore, useValue: mockTimelineStore },
        { provide: UiSettingsService, useValue: mockUiSettingsService }
      ]
    });
    
    service = TestBed.inject(ChatConnectionService);
  });

  // ... (existing tests remain)

  it('should emit forkChat command', () => {
    service.forkChat('event-123');
    expect(mockTransportService.send).toHaveBeenCalled();
    const sentEvent = mockTransportService.send.mock.calls[0][0];
    expect(sentEvent.payload.case).toBe('forkChat');
    expect(sentEvent.payload.value.targetEventId).toBe('event-123');
  });

  it('should emit inlineEdit command', () => {
    service.inlineEdit('event-123', 'Updated prompt text');
    expect(mockTransportService.send).toHaveBeenCalled();
    const sentEvent = mockTransportService.send.mock.calls[0][0];
    expect(sentEvent.payload.case).toBe('inlineEdit');
    expect(sentEvent.payload.value.targetEventId).toBe('event-123');
    expect(sentEvent.payload.value.newText).toBe('Updated prompt text');
  });

  it('should emit switchBranch command and eagerly prepare UI', () => {
    service.switchBranch('feature-branch');
    
    // Eager UI updates
    expect(mockTimelineStore.coreChat()).toEqual([]);
    expect(mockTimelineStore.isLoadingHistory()).toBe(true);

    // Command emission
    expect(mockTransportService.send).toHaveBeenCalled();
    const sentEvent = mockTransportService.send.mock.calls[0][0];
    expect(sentEvent.payload.case).toBe('switchBranch');
    expect(sentEvent.payload.value.targetBranch).toBe('feature-branch');
  });

  it('should pass topology variables into timeline on sync_history', () => {
    const mockSyncHistoryEvent = { 
      type: 'sync_history', 
      recentEvents: [], 
      digests: {}, 
      focus: { primaryFocus: [], relatedContext: [], dormantLenses: [] },
      nextCursor: '', 
      hasMore: false,
      activeBranch: 'branch-x',
      branches: { 'branch-x': { parentBranch: 'main', forkPoint: '0' } }
    } as any;
    
    service['routeIncomingEvent'](mockSyncHistoryEvent);
    
    expect(mockTimelineStore.processSyncHistory).toHaveBeenCalledWith(
      [], '', false, 'branch-x', { 'branch-x': { parentBranch: 'main', forkPoint: '0' } }
    );
  });
});