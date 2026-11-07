import { ComponentFixture, TestBed } from '@angular/core/testing';
import { GraphCanvasComponent } from './graph-canvas.component';
import { describe, beforeEach, it, expect, vi } from 'vitest';

describe('GraphCanvasComponent', () => {
  let component: GraphCanvasComponent;
  let fixture: ComponentFixture<GraphCanvasComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [GraphCanvasComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(GraphCanvasComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should initialize with default zoom and pan', () => {
    expect(component.zoom()).toBe(1);
    expect(component.panX()).toBe(0);
    expect(component.panY()).toBe(0);
    expect(component.transformString()).toBe('translate(0px, 0px) scale(1)');
  });

  it('should reset view to default values', () => {
    component.zoom.set(2.5);
    component.panX.set(100);
    component.panY.set(200);

    component.resetView();

    expect(component.zoom()).toBe(1);
    expect(component.panX()).toBe(0);
    expect(component.panY()).toBe(0);
  });

  it('should begin dragging on mousedown with primary button', () => {
    const mouseEvent = new MouseEvent('mousedown', { button: 0, clientX: 50, clientY: 100 });
    component.onMouseDown(mouseEvent);
    
    expect((component as any).isDragging).toBe(true);
    expect((component as any).lastMouseX).toBe(50);
    expect((component as any).lastMouseY).toBe(100);
  });

  it('should ignore mousedown if not primary button', () => {
    const mouseEvent = new MouseEvent('mousedown', { button: 2, clientX: 50, clientY: 100 });
    component.onMouseDown(mouseEvent);
    
    expect((component as any).isDragging).toBe(false);
  });

  it('should update pan values on mousemove if dragging', () => {
    component.onMouseDown(new MouseEvent('mousedown', { button: 0, clientX: 100, clientY: 100 }));
    
    component.onMouseMove(new MouseEvent('mousemove', { clientX: 150, clientY: 80 }));

    expect(component.panX()).toBe(50); 
    expect(component.panY()).toBe(-20); 
  });

  it('should not update pan values on mousemove if not dragging', () => {
    component.onMouseMove(new MouseEvent('mousemove', { clientX: 150, clientY: 80 }));

    expect(component.panX()).toBe(0);
    expect(component.panY()).toBe(0);
  });

  it('should stop dragging on mouseup', () => {
    component.onMouseDown(new MouseEvent('mousedown', { button: 0, clientX: 100, clientY: 100 }));
    expect((component as any).isDragging).toBe(true);

    component.onMouseUp();
    expect((component as any).isDragging).toBe(false);
  });

  it('should calculate zoom and offset pan towards pointer on wheel event', () => {
    // Mock getBoundingClientRect for deterministic math
    const mockTarget = document.createElement('div');
    vi.spyOn(mockTarget, 'getBoundingClientRect').mockReturnValue({
      left: 0, top: 0, width: 1000, height: 1000,
      right: 1000, bottom: 1000, x: 0, y: 0, toJSON: () => { /* empty */ }
    });

    const wheelEvent = new WheelEvent('wheel', { 
      deltaY: -100, // scrolling up (zoom in)
      clientX: 500, 
      clientY: 500
    });
    
    Object.defineProperty(wheelEvent, 'currentTarget', { value: mockTarget });
    vi.spyOn(wheelEvent, 'preventDefault').mockImplementation(() => { /* empty */ });

    component.onWheel(wheelEvent);

    expect(component.zoom()).toBeGreaterThan(1);
    // Because we zoomed in centered on (500, 500), the canvas must pan negatively to keep the point under the mouse
    expect(component.panX()).toBeLessThan(0);
    expect(component.panY()).toBeLessThan(0);
  });
});