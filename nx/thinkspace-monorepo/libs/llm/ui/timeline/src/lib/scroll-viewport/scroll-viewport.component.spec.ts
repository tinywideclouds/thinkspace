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
      <llm-scroll-viewport [ceilingAnchor]="mockAnchor()" (loadPreviousPage)="onLoadPrevious()">
        <div class="mock-message" style="height: 100px;">Message 1</div>
        <div class="mock-message" style="height: 100px;">Message 2</div>
        <div class="mock-message" style="height: 1000px;">Massive Message 3</div>
      </llm-scroll-viewport>
    </div>
  `
})
class TestHostComponent {
  public mockAnchor = signal<HTMLElement | null>(null);
  public onLoadPrevious = vi.fn();
}

describe('ScrollViewportComponent', () => {
  let hostFixture: ComponentFixture<TestHostComponent>;
  let hostComponent: TestHostComponent;
  let viewportComponent: ScrollViewportComponent;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [TestHostComponent, ScrollViewportComponent]
    }).compileComponents();

    hostFixture = TestBed.createComponent(TestHostComponent);
    hostComponent = hostFixture.componentInstance;
    
    const viewportDebug = hostFixture.debugElement.query(By.directive(ScrollViewportComponent));
    viewportComponent = viewportDebug.componentInstance;

    hostFixture.detectChanges();
    await hostFixture.whenStable();
  });

  it('should create the component and its sentinels', () => {
    expect(viewportComponent).toBeTruthy();
    expect(viewportComponent.containerRef()).toBeTruthy();
    expect(viewportComponent.topSentinelRef()).toBeTruthy();
    expect(viewportComponent.bottomSentinelRef()).toBeTruthy();
  });

  it('should expose a scrollToBottom method that clears the unread indicator', () => {
    viewportComponent.unreadIndicatorActive.set(true);
    expect(viewportComponent.unreadIndicatorActive()).toBe(true);

    viewportComponent.scrollToBottom();

    expect(viewportComponent.unreadIndicatorActive()).toBe(false);
  });

  it('should conditionally render the New Messages pill', () => {
    const getPill = () => hostFixture.nativeElement.querySelector('button');

    viewportComponent.isAtBottom.set(true);
    viewportComponent.unreadIndicatorActive.set(false);
    hostFixture.detectChanges();
    expect(getPill()).toBeNull();

    viewportComponent.isAtBottom.set(false);
    viewportComponent.unreadIndicatorActive.set(false);
    hostFixture.detectChanges();
    expect(getPill()).toBeNull();

    viewportComponent.isAtBottom.set(false);
    viewportComponent.unreadIndicatorActive.set(true);
    hostFixture.detectChanges();
    
    const pill = getPill();
    expect(pill).toBeTruthy();
    expect(pill?.textContent).toContain('New Messages ↓');
  });

  it('should trigger scrollToBottom when the pill is clicked', () => {
    const scrollToBottomSpy = vi.spyOn(viewportComponent, 'scrollToBottom');
    
    viewportComponent.isAtBottom.set(false);
    viewportComponent.unreadIndicatorActive.set(true);
    hostFixture.detectChanges();

    const pill = hostFixture.nativeElement.querySelector('button');
    pill.click();

    expect(scrollToBottomSpy).toHaveBeenCalled();
  });
});