import { Injectable } from '@angular/core';
import { webSocket, WebSocketSubject } from 'rxjs/webSocket';
import { Observable } from 'rxjs';
import { WSEvent, WSEventSchema } from '@org/llm-core-protos';
import { fromJson, toJsonString } from '@bufbuild/protobuf';

@Injectable({ providedIn: 'root' })
export class TransportService {
  private socket$: WebSocketSubject<WSEvent> | null = null;

  /**
   * Establishes a WebSocket connection and returns an Observable of strictly typed WSEvent protos.
   * Utilizes custom serializers to bridge RxJS with Buf's Protobuf encoding.
   */
  public connect(url: string): Observable<WSEvent> {
    if (!this.socket$ || this.socket$.closed) {
      this.socket$ = webSocket<WSEvent>({
        url,
        // Inbound: Parse raw JSON from the server and hydrate it into a WSEvent class
        deserializer: (e: MessageEvent) => {
          try {
            const parsed = JSON.parse(e.data);
            return fromJson(WSEventSchema, parsed);
          } catch (err) {
            console.error('[TransportService] 🚨 Deserialization Error!', err);
            console.error('[TransportService] 📦 Raw string from Go:', e.data);
            throw err; // Re-throw to let RxJS handle the teardown
          }
        },
        serializer: (value: WSEvent) => {
          return toJsonString(WSEventSchema, value);
        },
        openObserver: {
          next: () => console.log(`[TransportService] Connected to ${url}`)
        },
        closeObserver: {
          next: () => console.log(`[TransportService] Disconnected from ${url}`)
        }
      });
    }
    return this.socket$.asObservable();
  }

  /**
   * Pushes a WSEvent through the active WebSocket connection.
   */
  public send(message: WSEvent): void {
    if (this.socket$ && !this.socket$.closed) {
      this.socket$.next(message);
    } else {
      console.error('[TransportService] Cannot send message: WebSocket is not open.');
    }
  }

  /**
   * Completes the RxJS subject, closing the underlying WebSocket connection.
   */
  public disconnect(): void {
    if (this.socket$) {
      this.socket$.complete();
      this.socket$ = null;
    }
  }
}