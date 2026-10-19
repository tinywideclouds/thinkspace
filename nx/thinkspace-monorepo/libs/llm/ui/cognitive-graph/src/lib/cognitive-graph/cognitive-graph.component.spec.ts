import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CognitiveGraphComponent } from './cognitive-graph.component';
import { CognitiveGraphStore } from '../cognitive-graph.store.spec';
import { signal } from '@angular/core';
import { describe, beforeEach, it, expect, vi } from 'vitest';

describe('CognitiveGraphComponent', () => {
  let component: CognitiveGraphComponent;
  let fixture: ComponentFixture<CognitiveGraphComponent>;
  let mockStore: any;

  beforeEach(async () => {
    mockStore = {
      stickyNodes: signal([{ id: 'sticky-1', summary: 'Global architecture rule', isSticky: true }]),
      trajectoryNodes: signal([{ id: 'traj-1', summary: 'Refactored auth module', isSticky: false }]),
      activeFeed: signal([{ id: 'msg-1', source: 'user', content: 'What next?' }]),
      flowStates: signal(new Map()),
      isNodeInFocus: vi.fn((summary: string) => false),
      inspectFlow: vi.fn()
    };

    await TestBed.configureTestingModule({
      imports: [CognitiveGraphComponent],
      providers: [
        { provide: CognitiveGraphStore, useValue: mockStore }
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