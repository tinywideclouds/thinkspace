import { TestBed } from '@angular/core/testing';
import { TransportService } from './transport.service';
import { create } from '@bufbuild/protobuf';
import { WSEventSchema } from '@org/llm-core-protos';
import { take } from 'rxjs/operators';
import { firstValueFrom, Subscription } from 'rxjs';
import { describe, beforeEach, afterEach, it, expect, vi, Mock } from 'vitest';

// RxJS binds directly to these properties, so we must expose them in the mock
interface MockWebSocket {
  send: Mock;
  close: Mock;
  readyState: 0 | 1 | 2 | 3;
  onmessage: ((ev: any) => any) | null;
  onopen: ((ev: any) => any) | null;
  onclose: ((ev: any) => any) | null;
  onerror: ((ev: any) => any) | null;
}

describe('TransportService', () => {
  let service: TransportService;
  let mockWebSocketInstance: MockWebSocket;
  let sub: Subscription | null = null;

  beforeEach(() => {
    mockWebSocketInstance = {
      send: vi.fn(),
      close: vi.fn(),
      readyState: 1, // WebSocket.OPEN
      onmessage: null,
      onopen: null,
      onclose: null,
      onerror: null,
    };

    class DummyWebSocket {
      constructor() {
        return mockWebSocketInstance;
      }
    }

    vi.stubGlobal('WebSocket', DummyWebSocket);

    TestBed.configureTestingModule({
      providers: [TransportService]
    });
    service = TestBed.inject(TransportService);
  });

  afterEach(() => {
    if (sub) {
      sub.unsubscribe();
      sub = null;
    }
    vi.unstubAllGlobals();
    service.disconnect();
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });

  it('should deserialize inbound JSON to a WSEvent proto', async () => {
    // 1. Subscribe to force RxJS to instantiate the socket
    const eventPromise = firstValueFrom(
      service.connect('ws://localhost:8080/ws').pipe(take(1))
    );

    // 2. Simulate the socket opening (RxJS buffers messages until open)
    if (mockWebSocketInstance.onopen) {
      mockWebSocketInstance.onopen({ type: 'open' });
    }

    // 3. Simulate an inbound payload from Go
    if (mockWebSocketInstance.onmessage) {
      mockWebSocketInstance.onmessage({
        data: JSON.stringify({
          chatStream: { text: 'Inbound test' }
        })
      });
    } else {
      throw new Error('RxJS did not attach a message listener (onmessage) to the WebSocket');
    }

    // 4. Verify translation
    const event = await eventPromise;
    expect(event.payload.case).toBe('chatStream');
    if (event.payload.case === 'chatStream') {
      expect(event.payload.value.text).toBe('Inbound test');
    }
  });

  it('should serialize outbound WSEvent proto to JSON', () => {
    // 1. Subscribe to trigger socket creation
    sub = service.connect('ws://localhost:8080/ws').subscribe();

    // 2. Mark socket as open so RxJS flushes its outbound queue
    if (mockWebSocketInstance.onopen) {
      mockWebSocketInstance.onopen({ type: 'open' });
    }

    const proto = create(WSEventSchema, {
      payload: {
        case: 'submitPrompt',
        value: { text: 'Outbound test', spaceId: 'golang' }
      }
    });

    service.send(proto);

    // 3. Verify it hit the mock's send method
    expect(mockWebSocketInstance.send).toHaveBeenCalledWith(
      JSON.stringify({
        submitPrompt: { text: 'Outbound test', spaceId: 'golang' }
      })
    );
  });

  it('should close the socket on disconnect', () => {
    // 1. Subscribe to instantiate
    sub = service.connect('ws://localhost:8080/ws').subscribe();
    
    // 2. Simulate the socket opening so RxJS registers it as fully connected
    if (mockWebSocketInstance.onopen) {
      mockWebSocketInstance.onopen({ type: 'open' });
    }

    // 3. Disconnect
    service.disconnect();
    
    // 4. Verify RxJS called close() on the mock
    expect(mockWebSocketInstance.close).toHaveBeenCalled();
  });
});