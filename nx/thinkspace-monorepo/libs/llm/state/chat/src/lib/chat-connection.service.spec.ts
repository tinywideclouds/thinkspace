import { TestBed } from '@angular/core/testing';
import { ChatConnectionService } from './chat-connection.service';
import { TransportService } from '@org/llm-core-transport';
import { TimelineStore } from './timeline.store';
import { FlowStore } from './flow.store';
import { CognitiveStore } from './cognitive.store';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { UiSettingsService } from './ui-settings.service';
import { of } from 'rxjs';
import { LlmFacade, DelegationStrategy } from '@org/llm-core-facade';
import { WSEventSchema } from '@org/llm-core-protos';
import { create } from '@bufbuild/protobuf';

describe('ChatConnectionService', () => {
  let service: ChatConnectionService;
  let transportMock: any;
  let timelineMock: any;
  let workspaceMock: any;
  let flowMock: any;
  let cognitiveMock: any;

  beforeEach(() => {
    transportMock = {
      connect: vi.fn(() => of()),
      disconnect: vi.fn(),
      send: vi.fn()
    };

    timelineMock = {
      appendCoreChat: vi.fn(),
      clearSession: vi.fn(),
      hasMoreHistory: vi.fn(() => false),
      isLoadingHistory: vi.fn(() => false),
      nextCursor: vi.fn(() => ''),
      clearThinkingIndicators: vi.fn(),
      injectFlowExecution: vi.fn(),
      updateFlowExecutionStatus: vi.fn(),
      processSyncHistory: vi.fn(),
      ingestEvent: vi.fn(),
      appendToLastModelMessage: vi.fn(),
      coreChat: { set: vi.fn() }
    };

    workspaceMock = {
      activeSpaceId: vi.fn(() => 'space-1'),
      activeChatId: vi.fn(() => 'chat-1')
    };

    flowMock = {
      resetTurn: vi.fn(),
      clearSession: vi.fn(),
      appendSystemLog: vi.fn(),
      processTelemetry: vi.fn()
    };

    cognitiveMock = {
      resetTurn: vi.fn(),
      clearSession: vi.fn(),
      pendingContextReview: { set: vi.fn() },
      pendingStrategy: { set: vi.fn() },
      pendingReviewBranch: Object.assign(vi.fn(() => 'branch-1'), { set: vi.fn() }),
      pendingTransition: { set: vi.fn() },
      activeFocus: { set: vi.fn() },
      activeDigests: { set: vi.fn() },
      activeWorkbench: { set: vi.fn() },
      autoApproveContext: vi.fn(() => false)
    };

    TestBed.configureTestingModule({
      providers: [
        ChatConnectionService,
        { provide: TransportService, useValue: transportMock },
        { provide: TimelineStore, useValue: timelineMock },
        { provide: ThinkspaceStateService, useValue: workspaceMock },
        { provide: FlowStore, useValue: flowMock },
        { provide: CognitiveStore, useValue: cognitiveMock },
        { provide: UiSettingsService, useValue: { pagination: () => ({ size: 50 }) } }
      ]
    });
    service = TestBed.inject(ChatConnectionService);
  });

  it('should submit a prompt and append a system thinking indicator', () => {
    service.submitPrompt('hello manager');
    
    expect(timelineMock.appendCoreChat).toHaveBeenCalledWith('prompt', 'hello manager');
    expect(timelineMock.appendCoreChat).toHaveBeenCalledWith('system', 'Manager Thinking...');
    expect(transportMock.send).toHaveBeenCalled();
  });

  it('should submit context approval and clear thinking indicators on abort', () => {
    service.submitContextApproval(false);
    expect(timelineMock.clearThinkingIndicators).toHaveBeenCalled();
    expect(transportMock.send).toHaveBeenCalled();
  });

  it('should map telemetry events to flow execution UI methods', () => {
    // Expose the private method for testing the router switch directly
    const routeEvent = (service as any).routeIncomingEvent.bind(service);
    
    routeEvent({
      type: 'telemetry_event',
      channel: 'manager',
      id: 'uuid-1',
      eventType: 'flow_start',
      timestamp: Temporal.Now.instant(),
      summary: 'Orchestrating agents',
      payload: { flow_id: 'flow-123' }
    });

    expect(flowMock.processTelemetry).toHaveBeenCalled();
    expect(timelineMock.injectFlowExecution).toHaveBeenCalledWith('flow-123', 'Orchestrating agents');

    routeEvent({
      type: 'telemetry_event',
      channel: 'manager',
      id: 'uuid-2',
      eventType: 'flow_complete',
      timestamp: Temporal.Now.instant(),
      summary: '',
      payload: { flow_id: 'flow-123' }
    });

    expect(timelineMock.updateFlowExecutionStatus).toHaveBeenCalledWith('flow-123', 'completed');
  });

  it('should format switchBranch correctly and clear the timeline', () => {
    service.switchBranch('feature-v2');
    
    expect(timelineMock.coreChat.set).toHaveBeenCalledWith([]);
    expect(timelineMock.isLoadingHistory.set).toHaveBeenCalledWith(true);
    expect(transportMock.send).toHaveBeenCalled();
  });
});