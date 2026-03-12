import { useEffect, useRef, useState, useCallback } from 'react';

export type WebSocketState = 'connecting' | 'open' | 'closing' | 'closed';

export interface UseReconnectingWebSocketOptions {
  /** WebSocket URL to connect to */
  url: string;
  /** Whether to attempt connection (set to false to disable) */
  enabled?: boolean;
  /** Maximum number of reconnection attempts (0 = infinite) */
  maxRetries?: number;
  /** Initial backoff delay in milliseconds (default: 1000) */
  initialBackoff?: number;
  /** Maximum backoff delay in milliseconds (default: 30000) */
  maxBackoff?: number;
  /** Callback when a message is received */
  onMessage?: (event: MessageEvent) => void;
  /** Callback when connection opens */
  onOpen?: () => void;
  /** Callback when connection closes */
  onClose?: () => void;
  /** Callback when an error occurs */
  onError?: (error: Event) => void;
}

export interface UseReconnectingWebSocketReturn {
  /** Current WebSocket connection state */
  state: WebSocketState;
  /** Number of reconnection attempts made */
  retryCount: number;
  /** Whether the max retry limit has been reached */
  maxRetriesReached: boolean;
  /** Manually trigger a reconnection attempt */
  reconnect: () => void;
  /** Close the WebSocket connection and stop reconnecting */
  close: () => void;
}

/**
 * A React hook that manages a WebSocket connection with automatic reconnection.
 *
 * Features:
 * - Automatic reconnection with exponential backoff (1s → 30s max by default)
 * - Connection state tracking (connecting/open/closing/closed)
 * - Retry counter and max retry limit
 * - Proper cleanup on unmount
 * - Resets backoff to initial value on successful connection
 *
 * @example
 * ```tsx
 * const { state, retryCount, reconnect } = useReconnectingWebSocket({
 *   url: 'wss://example.com/socket',
 *   enabled: true,
 *   onMessage: (event) => {
 *     const data = JSON.parse(event.data);
 *     console.log('Received:', data);
 *   },
 * });
 *
 * if (state === 'closed') {
 *   return <button onClick={reconnect}>Reconnect</button>;
 * }
 * ```
 */
export function useReconnectingWebSocket(
  options: UseReconnectingWebSocketOptions
): UseReconnectingWebSocketReturn {
  const {
    url,
    enabled = true,
    maxRetries = 0,
    initialBackoff = 1000,
    maxBackoff = 30000,
    onMessage,
    onOpen,
    onClose,
    onError,
  } = options;

  const [state, setState] = useState<WebSocketState>('closed');
  const [retryCount, setRetryCount] = useState(0);
  const [maxRetriesReached, setMaxRetriesReached] = useState(false);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const currentBackoffRef = useRef(initialBackoff);
  const shouldReconnectRef = useRef(true);
  const unmountedRef = useRef(false);

  // Calculate next backoff delay with exponential growth
  const getNextBackoff = useCallback((): number => {
    const current = currentBackoffRef.current;
    const next = Math.min(current * 2, maxBackoff);
    currentBackoffRef.current = next;
    return current;
  }, [maxBackoff]);

  // Reset backoff to initial value (called on successful connection)
  const resetBackoff = useCallback(() => {
    currentBackoffRef.current = initialBackoff;
    setRetryCount(0);
    setMaxRetriesReached(false);
  }, [initialBackoff]);

  // Close WebSocket and cancel any pending reconnection
  const close = useCallback(() => {
    shouldReconnectRef.current = false;

    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    setState('closed');
  }, []);

  // Attempt to establish WebSocket connection
  const connect = useCallback(() => {
    // Don't connect if disabled, unmounted, or already connecting/open
    if (!enabled || unmountedRef.current || !shouldReconnectRef.current) {
      return;
    }

    if (wsRef.current?.readyState === WebSocket.CONNECTING ||
        wsRef.current?.readyState === WebSocket.OPEN) {
      return;
    }

    // Check max retries
    if (maxRetries > 0 && retryCount >= maxRetries) {
      setMaxRetriesReached(true);
      setState('closed');
      return;
    }

    setState('connecting');

    try {
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onopen = () => {
        if (unmountedRef.current) {
          ws.close();
          return;
        }

        setState('open');
        resetBackoff();
        onOpen?.();
      };

      ws.onmessage = (event) => {
        if (!unmountedRef.current) {
          onMessage?.(event);
        }
      };

      ws.onerror = (error) => {
        if (!unmountedRef.current) {
          onError?.(error);
        }
      };

      ws.onclose = () => {
        if (unmountedRef.current) {
          return;
        }

        wsRef.current = null;
        setState('closed');
        onClose?.();

        // Attempt reconnection if still enabled and should reconnect
        if (enabled && shouldReconnectRef.current) {
          const backoff = getNextBackoff();
          setRetryCount((prev) => prev + 1);

          reconnectTimeoutRef.current = window.setTimeout(() => {
            if (!unmountedRef.current && shouldReconnectRef.current) {
              connect();
            }
          }, backoff);
        }
      };
    } catch (error) {
      console.error('WebSocket connection error:', error);
      setState('closed');

      // Retry on exception
      if (enabled && shouldReconnectRef.current) {
        const backoff = getNextBackoff();
        setRetryCount((prev) => prev + 1);

        reconnectTimeoutRef.current = window.setTimeout(() => {
          if (!unmountedRef.current && shouldReconnectRef.current) {
            connect();
          }
        }, backoff);
      }
    }
  }, [enabled, url, maxRetries, retryCount, onOpen, onMessage, onError, onClose, getNextBackoff, resetBackoff]);

  // Manual reconnect function (resets retry count)
  const reconnect = useCallback(() => {
    shouldReconnectRef.current = true;
    resetBackoff();
    close();

    // Small delay to allow close to complete
    window.setTimeout(() => {
      if (!unmountedRef.current) {
        connect();
      }
    }, 100);
  }, [close, connect, resetBackoff]);

  // Initial connection and re-connection when URL or enabled changes
  useEffect(() => {
    unmountedRef.current = false;
    shouldReconnectRef.current = true;

    if (enabled) {
      connect();
    } else {
      close();
    }

    return () => {
      unmountedRef.current = true;
      shouldReconnectRef.current = false;

      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }

      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [enabled, url, connect, close]);

  return {
    state,
    retryCount,
    maxRetriesReached,
    reconnect,
    close,
  };
}
