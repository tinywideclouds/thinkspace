import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatStrategyPromptComponent } from './chat-strategy-prompt.component';
import { DelegationStrategy } from '@org/llm-core-facade';
import { vi, describe, beforeEach, it, expect } from 'vitest';

describe('ChatStrategyPromptComponent', () => {
  let component: ChatStrategyPromptComponent;
  let fixture: ComponentFixture<ChatStrategyPromptComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatStrategyPromptComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(ChatStrategyPromptComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should emit the selected strategy with autoAccept false by default', () => {
    const emitSpy = vi.spyOn(component.strategySelected, 'emit');

    component.select(DelegationStrategy.REFINE);

    expect(emitSpy).toHaveBeenCalledWith({
      strategy: DelegationStrategy.REFINE,
      autoAccept: false,
    });
  });

  it('should emit autoAccept true when toggled before selecting REFINE', () => {
    const emitSpy = vi.spyOn(component.strategySelected, 'emit');
    const checkbox = fixture.nativeElement.querySelector('input[type="checkbox"]') as HTMLInputElement;

    checkbox.checked = true;
    checkbox.dispatchEvent(new Event('change'));
    fixture.detectChanges();

    component.select(DelegationStrategy.REFINE);

    expect(emitSpy).toHaveBeenCalledWith({
      strategy: DelegationStrategy.REFINE,
      autoAccept: true,
    });
  });

  it('should display a verification warning when hasVerificationFailure is true', () => {
    expect((fixture.nativeElement as HTMLElement).textContent).not.toContain(
      'Sub-agent verification failed',
    );

    fixture.componentRef.setInput('hasVerificationFailure', true);
    fixture.detectChanges();

    expect((fixture.nativeElement as HTMLElement).textContent).toContain(
      'Sub-agent verification failed. Review required.',
    );
  });
});