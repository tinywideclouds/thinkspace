import { Component, signal, computed } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'llm-graph-canvas',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './graph-canvas.component.html',
  styleUrl: './graph-canvas.component.scss',
  host: {
    class: 'block w-full h-full'
  }
})
export class GraphCanvasComponent {
  public zoom = signal<number>(1);
  public panX = signal<number>(0);
  public panY = signal<number>(0);

  private isDragging = false;
  private lastMouseX = 0;
  private lastMouseY = 0;

  public MIN_ZOOM = 0.1;
  public MAX_ZOOM = 3.0;
  public ZOOM_SENSITIVITY = 0.002;

  public transformString = computed(() => {
    return `translate(${this.panX()}px, ${this.panY()}px) scale(${this.zoom()})`;
  });

  public onWheel(event: WheelEvent): void {
    event.preventDefault();

    const currentZoom = this.zoom();
    const zoomDelta = -event.deltaY * this.ZOOM_SENSITIVITY;
    let newZoom = currentZoom + (currentZoom * zoomDelta);
    newZoom = Math.max(this.MIN_ZOOM, Math.min(newZoom, this.MAX_ZOOM));

    const targetElement = event.currentTarget as HTMLElement;
    const rect = targetElement.getBoundingClientRect();
    
    const mouseX = event.clientX - rect.left;
    const mouseY = event.clientY - rect.top;

    const scaleChange = newZoom - currentZoom;

    const currentPanX = this.panX();
    const currentPanY = this.panY();

    const newPanX = currentPanX - ((mouseX - currentPanX) / currentZoom) * scaleChange;
    const newPanY = currentPanY - ((mouseY - currentPanY) / currentZoom) * scaleChange;

    this.zoom.set(newZoom);
    this.panX.set(newPanX);
    this.panY.set(newPanY);
  }

  public onMouseDown(event: MouseEvent): void {
    if (event.button !== 0) return; 
    
    this.isDragging = true;
    this.lastMouseX = event.clientX;
    this.lastMouseY = event.clientY;
    
    document.body.style.cursor = 'grabbing';
  }

  public onMouseMove(event: MouseEvent): void {
    if (!this.isDragging) return;

    const deltaX = event.clientX - this.lastMouseX;
    const deltaY = event.clientY - this.lastMouseY;

    this.panX.update(x => x + deltaX);
    this.panY.update(y => y + deltaY);

    this.lastMouseX = event.clientX;
    this.lastMouseY = event.clientY;
  }

  public onMouseUp(): void {
    if (!this.isDragging) return;
    this.isDragging = false;
    document.body.style.cursor = 'default';
  }

  public resetView(): void {
    this.zoom.set(1);
    this.panX.set(0);
    this.panY.set(0);
  }
}