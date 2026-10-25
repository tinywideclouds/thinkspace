import { ComponentFixture, TestBed, fakeAsync, tick } from '@angular/core/testing';
import { ComponentRef } from '@angular/core';
import { FileDeltaBlockComponent } from './file-delta-block.component';
import { TimelineFacade } from '../../timeline.facade';
import { describe, beforeEach, it, expect, vi } from 'vitest';

describe('FileDeltaBlockComponent', () => {
  let component: FileDeltaBlockComponent;
  let fixture: ComponentFixture<FileDeltaBlockComponent>;
  let componentRef: ComponentRef<FileDeltaBlockComponent>;
  let mockFacade: { getFileContent: any };

  beforeEach(async () => {
    mockFacade = {
      getFileContent: vi.fn().mockResolvedValue('console.log("Full file fetched!");')
    };

    // Stub scrollIntoView which is not implemented in JSDOM
    window.HTMLElement.prototype.scrollIntoView = vi.fn();

    await TestBed.configureTestingModule({
      imports: [FileDeltaBlockComponent],
      providers: [
        { provide: TimelineFacade, useValue: mockFacade }
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(FileDeltaBlockComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;

    componentRef.setInput('path', 'src/main.ts');
    componentRef.setInput('summary', 'Added initialization logic');
    componentRef.setInput('previewSnippet', 'console.log("Preview");');
    componentRef.setInput('fileRef', 'hash123');
    
    await fixture.whenStable();
    fixture.detectChanges();
  });

  it('should default to collapsed state and show the preview snippet', () => {
    expect(component.isExpanded()).toBe(false);
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('console.log("Preview");');
  });

  it('should fetch full content, expand, and trigger scrollIntoView when header is clicked', fakeAsync(() => {
    const scrollSpy = vi.spyOn(component.containerRef().nativeElement, 'scrollIntoView');
    
    const header = fixture.nativeElement.querySelector('div[role="button"]');
    header.click();
    
    expect(component.isExpanded()).toBe(true);
    expect(component.isLoading()).toBe(true);
    
    tick(50); // Account for the setTimeout scroll tick
    fixture.detectChanges();
    
    expect(scrollSpy).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' });
    expect(component.isLoading()).toBe(false);
    expect(component.fullContent()).toBe('console.log("Full file fetched!");');
    expect(mockFacade.getFileContent).toHaveBeenCalledWith('src/main.ts', 'hash123');
  }));

  it('should trigger expand when Enter key is pressed on header', fakeAsync(() => {
    const header = fixture.nativeElement.querySelector('div[role="button"]');
    const enterEvent = new KeyboardEvent('keydown', { key: 'Enter' });
    header.dispatchEvent(enterEvent);
    
    expect(component.isExpanded()).toBe(true);
    expect(component.isLoading()).toBe(true);

    tick(50);
    fixture.detectChanges();

    expect(component.isLoading()).toBe(false);
    expect(component.fullContent()).toBe('console.log("Full file fetched!");');
  }));
});