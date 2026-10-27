import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Component, signal } from '@angular/core';
import { ScrollViewportComponent } from './scroll-viewport.component';
import { describe, beforeEach, it, expect, vi } from 'vitest';
import { By } from '@angular/platform-browser';

@Component({
  standalone: true,
  imports: [ScrollViewportComponent],
  template: `
    <div style="height: 500px; width: 400px;">
      <llm-scroll-viewport [topContentAnchor]="mockAnchor()" [prefetchAnchor]="prefetchMockAnchor()" (loadPreviousPage)="onLoadPrevious()">
        <div class="mock-message" style="height: 100px;">Message 1</div>
        <div class="mock-message" style="height: 100px;">Message 2</div>
        <div class="mock-message" style="height: 1000px;">Massive Message 3</div>
      </llm-scroll-viewport>
    </div>
  `
})
class TestHostComponent {
  public mockAnchor = signal<HTMLElement | null>(null);
  public prefetchMockAnchor = signal<HTMLElement | null>(null);
  public onLoadPrevious = vi.fn();
}

describe('ScrollViewportComponent', () => {
  let hostFixture: ComponentFixture<TestHostComponent>;
  let viewportComponent: ScrollViewportComponent;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [TestHostComponent, ScrollViewportComponent]
    }).compileComponents();

    hostFixture = TestBed.createComponent(TestHostComponent);
    
    const viewportDebug = hostFixture.debugElement.query(By.directive(ScrollViewportComponent));
    viewportComponent = viewportDebug.componentInstance;

    hostFixture.detectChanges();
    await hostFixture.whenStable();
  });

  it('should create the component and its bottom sentinel', () => {
    expect(viewportComponent).toBeTruthy();
    expect(viewportComponent.containerRef()).toBeTruthy();
    expect(viewportComponent.bottomSentinelRef()).toBeTruthy();
  });

  it('should expose a scrollToBottom method and emit scrolledToBottom, updating scroll state to auto', () => {
    const emitSpy = vi.spyOn(viewportComponent.scrolledToBottom, 'emit');
    
    viewportComponent.scrollState.set('manual');
    viewportComponent.scrollToBottom('instant');

    expect(viewportComponent.scrollState()).toBe('auto');
    expect(emitSpy).toHaveBeenCalled();
  });

  it('should only maintain tail if scroll state is auto', () => {
    viewportComponent.scrollState.set('manual');
    const container = viewportComponent.containerRef().nativeElement;
    container.scrollTop = 0;

    viewportComponent['enforceTailLock']();
    
    expect(container.scrollTop).toBe(0);

    viewportComponent.scrollState.set('auto');
    viewportComponent['enforceTailLock']();
    
    expect(container.scrollTop).toBe(container.scrollHeight);
  });
});