import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ComponentRef, signal, WritableSignal } from '@angular/core';
import { FileDeltaBlockComponent } from './file-delta-block.component';
import { TimelineFacade } from '../../timeline.facade';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';

// Define the full strict interface expected by the nested component
interface MockTimelineFacade {
  getFileContent: Mock<(path: string, ref: string) => Promise<string>>;
  isDebugMode: WritableSignal<boolean>;
  isEditMode: WritableSignal<boolean>;
}

describe('FileDeltaBlockComponent', () => {
  let component: FileDeltaBlockComponent;
  let fixture: ComponentFixture<FileDeltaBlockComponent>;
  let componentRef: ComponentRef<FileDeltaBlockComponent>;
  let mockFacade: MockTimelineFacade;

  beforeEach(async () => {
    // Populate every method and signal the component depends on
    mockFacade = {
      getFileContent: vi.fn().mockResolvedValue('console.log("Full file fetched!");'),
      isDebugMode: signal(false),
      isEditMode: signal(false)
    };

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

  it('should fetch full content and expand when header is clicked', async () => {
    const header = fixture.nativeElement.querySelector('div[role="button"]') as HTMLElement;
    
    header.click();
    
    await fixture.whenStable();
    fixture.detectChanges();
    
    expect(component.isExpanded()).toBe(true);
    expect(component.isLoading()).toBe(false);
    expect(component.fullContent()).toBe('console.log("Full file fetched!");');
    expect(mockFacade.getFileContent).toHaveBeenCalledWith('src/main.ts', 'hash123');
  });

  it('should trigger expand when Enter key is pressed on header', async () => {
    const header = fixture.nativeElement.querySelector('div[role="button"]') as HTMLElement;
    const enterEvent = new KeyboardEvent('keydown', { key: 'Enter' });
    
    header.dispatchEvent(enterEvent);
    
    await fixture.whenStable();
    fixture.detectChanges();

    expect(component.isExpanded()).toBe(true);
    expect(component.isLoading()).toBe(false);
    expect(component.fullContent()).toBe('console.log("Full file fetched!");');
  });
});