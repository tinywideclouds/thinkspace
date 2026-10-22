import { ComponentFixture, TestBed } from '@angular/core/testing';
import { AppComponent } from './app';
import { ChatConnectionService } from '@org/llm-state-chat';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';

interface MockConnectionService {
  connect: Mock<(url: string) => void>;
}

describe('AppComponent', () => {
  let component: AppComponent;
  let fixture: ComponentFixture<AppComponent>;
  let mockConnectionService: MockConnectionService;

  beforeEach(async () => {
    mockConnectionService = { 
      connect: vi.fn() 
    };

    await TestBed.configureTestingModule({
      imports: [AppComponent],
      providers: [
        { provide: ChatConnectionService, useValue: mockConnectionService }
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(AppComponent);
    component = fixture.componentInstance;
  });

  it('should create the app', () => {
    expect(component).toBeTruthy();
  });

  it('should connect to the websocket after next render', async () => {
    fixture.detectChanges();
    await Promise.resolve();
    expect(mockConnectionService.connect).toHaveBeenCalledWith('ws://localhost:8080/ws');
  });
});