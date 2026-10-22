import { ComponentFixture, TestBed } from '@angular/core/testing';
import { DeskComponent } from './desk.component';
import { ComponentRef } from '@angular/core';
import { describe, beforeEach, it, expect } from 'vitest';
import { FileContextMetaData } from '@org/llm-core-facade';

describe('DeskComponent', () => {
  let component: DeskComponent;
  let fixture: ComponentFixture<DeskComponent>;
  let componentRef: ComponentRef<DeskComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [DeskComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(DeskComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;

    const mockItems: FileContextMetaData[] = [
      { path: 'src/main.ts', level: 'structural', tokenCount: 150 }
    ];

    componentRef.setInput('items', mockItems);
    
    await fixture.whenStable();
    fixture.detectChanges();
  });

  it('should create the component', () => {
    expect(component).toBeTruthy();
  });
});