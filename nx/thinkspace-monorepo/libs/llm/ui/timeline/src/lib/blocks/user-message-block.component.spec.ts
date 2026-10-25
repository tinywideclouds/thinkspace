import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ComponentRef } from '@angular/core';
import { UserMessageBlockComponent } from './user-message-block.component';
import { describe, beforeEach, it, expect } from 'vitest';

describe('UserMessageBlockComponent', () => {
  let component: UserMessageBlockComponent;
  let fixture: ComponentFixture<UserMessageBlockComponent>;
  let componentRef: ComponentRef<UserMessageBlockComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [UserMessageBlockComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(UserMessageBlockComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;

    componentRef.setInput('content', 'Hello World');
    
    await fixture.whenStable();
    fixture.detectChanges();
  });

  it('should create the component', () => {
    expect(component).toBeTruthy();
  });

  it('should render the provided content', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Hello World');
    expect(compiled.textContent).toContain('USER');
  });
});