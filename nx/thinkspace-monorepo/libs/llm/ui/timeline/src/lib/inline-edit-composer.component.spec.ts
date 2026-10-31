import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ComponentRef } from '@angular/core';
import { InlineEditComposerComponent } from './inline-edit-composer.component';
import { describe, beforeEach, it, expect, vi } from 'vitest';

describe('InlineEditComposerComponent', () => {
  let component: InlineEditComposerComponent;
  let fixture: ComponentFixture<InlineEditComposerComponent>;
  let componentRef: ComponentRef<InlineEditComposerComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [InlineEditComposerComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(InlineEditComposerComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;

    componentRef.setInput('initialText', 'Original prompt');
    await fixture.whenStable();
    fixture.detectChanges();
  });

  it('should initialize with the provided text', () => {
    const textarea = fixture.nativeElement.querySelector('textarea');
    expect(component.editText()).toBe('Original prompt');
    expect(textarea.value).toBe('Original prompt');
  });

  it('should emit save with new text when Save is clicked', () => {
    const emitSpy = vi.spyOn(component.save, 'emit');
    component.editText.set('Updated prompt');
    fixture.detectChanges();

    const saveButton = fixture.nativeElement.querySelectorAll('button')[1];
    saveButton.click();

    expect(emitSpy).toHaveBeenCalledWith('Updated prompt');
  });

  it('should disable save button if text has not changed', () => {
    const saveButton = fixture.nativeElement.querySelectorAll('button')[1];
    expect(saveButton.disabled).toBe(true);
  });

  it('should emit exit when Cancel is clicked', () => {
    const emitSpy = vi.spyOn(component.exit, 'emit');
    const cancelButton = fixture.nativeElement.querySelectorAll('button')[0];
    cancelButton.click();

    expect(emitSpy).toHaveBeenCalled();
  });
});