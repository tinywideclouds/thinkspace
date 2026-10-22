import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CognitiveGraphComponent } from './cognitive-graph.component';
import { CognitiveGraphFacade } from '../cognitive-graph.facade';
import { signal, WritableSignal } from '@angular/core';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';
import { ChatItem, FlowState } from '@org/llm-state-chat';
import { DomainDigestMeta } from '@org/llm-core-facade';

interface MockCognitiveGraphFacade {
  stickyNodes: WritableSignal<DomainDigestMeta[]>;
  trajectoryNodes: WritableSignal<DomainDigestMeta[]>;
  activeFeed: WritableSignal<ChatItem[]>;
  flowStates: WritableSignal<Map<string, FlowState>>;
  isNodeInFocus: Mock<(summary: string) => boolean>;
  inspectFlow: Mock<(flowId: string) => void>;
}

describe('CognitiveGraphComponent', () => {
  let component: CognitiveGraphComponent;
  let fixture: ComponentFixture<CognitiveGraphComponent>;
  let mockFacade: MockCognitiveGraphFacade;

  beforeEach(async () => {
    mockFacade = {
      stickyNodes: signal([
        { id: 'sticky-1', summary: 'Global architecture rule', isSticky: true }
      ]),
      trajectoryNodes: signal([
        { id: 'traj-1', summary: 'Refactored auth module', isSticky: false }
      ]),
      activeFeed: signal([
        { id: 'msg-1', source: 'user', content: 'What next?' }
      ]),
      flowStates: signal(new Map<string, FlowState>()),
      isNodeInFocus: vi.fn((summary: string) => false),
      inspectFlow: vi.fn()
    };

    await TestBed.configureTestingModule({
      imports: [CognitiveGraphComponent],
      providers: [
        { provide: CognitiveGraphFacade, useValue: mockFacade }
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(CognitiveGraphComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should render the three core tracks: Anchors, Trajectory, and Horizon Working Memory', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    
    // Check Sticky Track
    expect(compiled.textContent).toContain('Core Anchors');
    expect(compiled.textContent).toContain('Global architecture rule');
    
    // Check Trajectory Track
    expect(compiled.textContent).toContain('Cognitive Trajectory');
    expect(compiled.textContent).toContain('Refactored auth module');
    
    // Check Horizon and Working Memory
    expect(compiled.textContent).toContain('Digestion Horizon');
    expect(compiled.textContent).toContain('What next?');
  });
});