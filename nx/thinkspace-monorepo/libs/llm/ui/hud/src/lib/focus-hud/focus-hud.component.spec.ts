import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FocusHudComponent } from './focus-hud.component';
import { ComponentRef } from '@angular/core';
import { describe, beforeEach, it, expect, vi } from 'vitest';
import { DomainAttentionFocus } from '@org/llm-core-facade';

describe('FocusHudComponent', () => {
  let component: FocusHudComponent;
  let fixture: ComponentFixture<FocusHudComponent>;
  let componentRef: ComponentRef<FocusHudComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [FocusHudComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(FocusHudComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;

    const mockFocus: DomainAttentionFocus = {
      primaryFocus: ['architecture'],
      relatedContext: ['auth'],
      dormantLenses: ['database']
    };

    componentRef.setInput('focus', mockFocus);
    componentRef.setInput('activeHoverTag', null);
    
    fixture.detectChanges();
  });

  it('should create the HUD component', () => {
    expect(component).toBeTruthy();
  });

  it('should accept focus and activeHoverTag inputs', () => {
    expect(component.focus()?.primaryFocus).toContain('architecture');
    expect(component.activeHoverTag()).toBeNull();
    
    componentRef.setInput('activeHoverTag', 'auth');
    fixture.detectChanges();
    
    expect(component.activeHoverTag()).toBe('auth');
  });

  it('should successfully emit tagHovered events', () => {
    const emitSpy = vi.spyOn(component.tagHovered, 'emit');
    
    component.tagHovered.emit('auth');
    expect(emitSpy).toHaveBeenCalledWith('auth');
    
    component.tagHovered.emit(null);
    expect(emitSpy).toHaveBeenCalledWith(null);
  });

  it('should successfully emit tagClicked events', () => {
    const emitSpy = vi.spyOn(component.tagClicked, 'emit');
    
    component.tagClicked.emit('architecture');
    expect(emitSpy).toHaveBeenCalledWith('architecture');
  });
});