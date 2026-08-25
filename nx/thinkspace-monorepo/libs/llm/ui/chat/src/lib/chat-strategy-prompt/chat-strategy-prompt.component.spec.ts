import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatStrategyPromptComponent } from './chat-strategy-prompt.component';
import { DomainDelegationStrategy } from '@org/llm-core-facade';
import { vi, describe, beforeEach, it, expect } from 'vitest';

describe('ChatStrategyPromptComponent', () => {
  let component: ChatStrategyPromptComponent;
  let fixture: ComponentFixture<ChatStrategyPromptComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatStrategyPromptComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatStrategyPromptComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should emit the selected strategy', () => {
    const emitSpy = vi.spyOn(component.strategySelected, 'emit');
    
    component.select(DomainDelegationStrategy.REFINE);
    
    expect(emitSpy).toHaveBeenCalledWith(DomainDelegationStrategy.REFINE);
  });
});