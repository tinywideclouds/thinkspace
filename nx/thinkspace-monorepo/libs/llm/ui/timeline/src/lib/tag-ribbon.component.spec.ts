import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ComponentRef } from '@angular/core';
import { TagRibbonComponent } from './tag-ribbon.component';
import { describe, beforeEach, it, expect } from 'vitest';

describe('TagRibbonComponent', () => {
  let component: TagRibbonComponent;
  let fixture: ComponentFixture<TagRibbonComponent>;
  let componentRef: ComponentRef<TagRibbonComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [TagRibbonComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(TagRibbonComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;

    componentRef.setInput('tags', ['architecture', 'database']);
    
    await fixture.whenStable();
    fixture.detectChanges();
  });

  it('should create the component', () => {
    expect(component).toBeTruthy();
  });

  it('should deterministically generate a color class for a specific tag', () => {
    const colors = component.ribbonColors();
    expect(colors.length).toBe(2);
    
    // Test determinism: the same tag array should always produce the exact same color classes
    componentRef.setInput('tags', ['architecture', 'database']);
    fixture.detectChanges();
    
    expect(component.ribbonColors()).toEqual(colors);
  });

  it('should render a visual stripe for each tag', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    const stripes = compiled.querySelectorAll('.h-full.flex-1.rounded-full');
    
    expect(stripes.length).toBe(2);
  });

  it('should handle empty tags safely', () => {
    componentRef.setInput('tags', []);
    fixture.detectChanges();
    
    const compiled = fixture.nativeElement as HTMLElement;
    const stripes = compiled.querySelectorAll('.h-full.flex-1.rounded-full');
    
    expect(component.ribbonColors().length).toBe(0);
    expect(stripes.length).toBe(0);
  });
});