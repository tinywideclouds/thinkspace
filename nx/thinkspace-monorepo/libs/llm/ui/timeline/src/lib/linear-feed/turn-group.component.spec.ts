import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ComponentRef } from '@angular/core';
import { TurnGroupComponent } from './turn-group.component';
import { describe, beforeEach, it, expect, vi } from 'vitest';

describe('TurnGroupComponent', () => {
  let component: TurnGroupComponent;
  let fixture: ComponentFixture<TurnGroupComponent>;
  let componentRef: ComponentRef<TurnGroupComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [TurnGroupComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(TurnGroupComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;

    componentRef.setInput('isEditMode', false);
    componentRef.setInput('isSelected', false);
    componentRef.setInput('tags', []);
    
    await fixture.whenStable();
    fixture.detectChanges();
  });

  it('should not render a checkbox when not in edit mode', () => {
    const checkbox = fixture.nativeElement.querySelector('input[type="checkbox"]');
    expect(checkbox).toBeNull();
  });

  it('should render a checkbox and highlight the group when in edit mode', () => {
    componentRef.setInput('isEditMode', true);
    fixture.detectChanges();

    const checkbox = fixture.nativeElement.querySelector('input[type="checkbox"]');
    expect(checkbox).toBeTruthy();
    
    const wrapper = fixture.nativeElement.firstElementChild as HTMLElement;
    expect(wrapper.className).toContain('ring-2');
  });

  it('should emit selectionToggled when the checkbox is changed', () => {
    componentRef.setInput('isEditMode', true);
    fixture.detectChanges();

    const emitSpy = vi.spyOn(component.selectionToggled, 'emit');
    const checkbox = fixture.nativeElement.querySelector('input[type="checkbox"]') as HTMLInputElement;
    
    checkbox.dispatchEvent(new Event('change'));
    expect(emitSpy).toHaveBeenCalled();
  });

  it('should render the tag ribbon if tags are provided', () => {
    componentRef.setInput('tags', ['ui-refactor']);
    fixture.detectChanges();

    const ribbon = fixture.nativeElement.querySelector('llm-tag-ribbon');
    expect(ribbon).toBeTruthy();
  });

  it('should apply dimming classes if in edit mode but not selected', () => {
    componentRef.setInput('isEditMode', true);
    componentRef.setInput('isSelected', false);
    fixture.detectChanges();

    const wrapper = fixture.nativeElement.firstElementChild as HTMLElement;
    expect(wrapper.className).toContain('opacity-50');
    expect(wrapper.className).toContain('grayscale');
  });

  it('should not apply dimming classes if selected in edit mode', () => {
    componentRef.setInput('isEditMode', true);
    componentRef.setInput('isSelected', true);
    fixture.detectChanges();

    const wrapper = fixture.nativeElement.firstElementChild as HTMLElement;
    expect(wrapper.className).not.toContain('opacity-50');
    expect(wrapper.className).not.toContain('grayscale');
  });
});