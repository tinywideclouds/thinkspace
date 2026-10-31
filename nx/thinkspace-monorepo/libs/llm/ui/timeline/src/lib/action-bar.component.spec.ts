import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ComponentRef } from '@angular/core';
import { ActionBarComponent } from './action-bar.component';
import { describe, beforeEach, it, expect, vi } from 'vitest';

describe('ActionBarComponent', () => {
  let component: ActionBarComponent;
  let fixture: ComponentFixture<ActionBarComponent>;
  let componentRef: ComponentRef<ActionBarComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ActionBarComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(ActionBarComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;

    componentRef.setInput('selectedCount', 3);
    componentRef.setInput('position', 'bottom');
    
    await fixture.whenStable();
    fixture.detectChanges();
  });

  it('should display the selected turn count', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('3');
    expect(compiled.textContent).toContain('Turns Selected');
  });

  it('should apply the correct position class', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    const container = compiled.firstElementChild as HTMLElement;
    
    expect(container.className).toContain('bottom-6');
    expect(container.className).not.toContain('top-6');
  });

  it('should emit actionRequested with "tag" when Tag Context is clicked', () => {
    const emitSpy = vi.spyOn(component.actionRequested, 'emit');
    const buttons = fixture.nativeElement.querySelectorAll('button');
    
    buttons[0].click(); // Tag Context button
    expect(emitSpy).toHaveBeenCalledWith('tag');
  });

  it('should emit actionRequested with "tombstone" when Tombstone is clicked', () => {
    const emitSpy = vi.spyOn(component.actionRequested, 'emit');
    const buttons = fixture.nativeElement.querySelectorAll('button');
    
    buttons[1].click(); // Tombstone button
    expect(emitSpy).toHaveBeenCalledWith('tombstone');
  });

  it('should emit actionRequested with "fork" when Fork Branch is clicked', () => {
    const emitSpy = vi.spyOn(component.actionRequested, 'emit');
    const buttons = fixture.nativeElement.querySelectorAll('button');
    
    buttons[2].click(); // Fork Branch button
    expect(emitSpy).toHaveBeenCalledWith('fork');
  });

  it('should emit cancelRequested when Cancel is clicked', () => {
    const emitSpy = vi.spyOn(component.cancelRequested, 'emit');
    const buttons = fixture.nativeElement.querySelectorAll('button');
    
    buttons[3].click(); // Cancel button
    expect(emitSpy).toHaveBeenCalled();
  });
});