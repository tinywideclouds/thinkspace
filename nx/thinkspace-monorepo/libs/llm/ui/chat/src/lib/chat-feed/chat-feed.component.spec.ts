import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatFeedComponent } from './chat-feed.component';
import { ComponentRef } from '@angular/core';
import { describe, beforeEach, it, expect } from 'vitest';

describe('ChatFeedComponent', () => {
  let component: ChatFeedComponent;
  let fixture: ComponentFixture<ChatFeedComponent>;
  let componentRef: ComponentRef<ChatFeedComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatFeedComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatFeedComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;
    
    // Set required signal input
    componentRef.setInput('feed', [
      { id: '1', source: 'user', content: 'Test prompt' },
      { id: '2', source: 'model', content: 'Test response' }
    ]);
    fixture.detectChanges();
  });

  it('should create the component', () => {
    expect(component).toBeTruthy();
  });

  it('should render feed items', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    const strongTags = compiled.querySelectorAll('strong');
    
    expect(strongTags.length).toBe(2);
    expect(strongTags[0].textContent).toContain('user');
    expect(strongTags[1].textContent).toContain('model');
  });
});