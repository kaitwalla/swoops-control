import { useEffect, useRef, useCallback } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebLinksAddon } from '@xterm/addon-web-links';
import '@xterm/xterm/css/xterm.css';
import { useReconnectingWebSocket } from '../hooks/useReconnectingWebSocket';

interface TerminalOutputProps {
  /** Initial output to display */
  initialOutput?: string;
  /** Session ID for WebSocket streaming */
  sessionId?: string;
  /** Whether the session is active (enables live streaming) */
  isActive?: boolean;
  /** Callback when the user presses Enter in the terminal (not used yet) */
  onInput?: (data: string) => void;
}

export function TerminalOutput({ initialOutput, sessionId, isActive }: TerminalOutputProps) {
  const terminalRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const lastOutputRef = useRef<string>('');

  const writeOutput = useCallback((output: string) => {
    const term = termRef.current;
    if (!term || output === lastOutputRef.current) return;
    lastOutputRef.current = output;

    // Clear and rewrite (capture-pane gives full snapshots, not diffs)
    term.clear();
    // Split into lines and write
    const lines = output.split('\n');
    for (let i = 0; i < lines.length; i++) {
      if (i > 0) term.write('\r\n');
      term.write(lines[i]);
    }
  }, []);

  useEffect(() => {
    if (!terminalRef.current) return;

    const term = new Terminal({
      theme: {
        background: '#0a0a0a',
        foreground: '#d4d4d4',
        cursor: '#d4d4d4',
        selectionBackground: '#3b3b3b',
      },
      fontSize: 13,
      fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', Menlo, Monaco, Consolas, monospace",
      scrollback: 5000,
      cursorBlink: false,
      disableStdin: true,
      convertEol: true,
    });

    const fitAddon = new FitAddon();
    const webLinksAddon = new WebLinksAddon();

    term.loadAddon(fitAddon);
    term.loadAddon(webLinksAddon);
    term.open(terminalRef.current);

    // Delay fit to allow DOM to settle
    requestAnimationFrame(() => {
      try {
        fitAddon.fit();
      } catch {
        // ignore fit errors during unmount
      }
    });

    termRef.current = term;
    fitAddonRef.current = fitAddon;

    // Write initial output
    if (initialOutput) {
      writeOutput(initialOutput);
    }

    // Handle resize
    const observer = new ResizeObserver(() => {
      try {
        fitAddon.fit();
      } catch {
        // ignore
      }
    });
    observer.observe(terminalRef.current);

    return () => {
      observer.disconnect();
      term.dispose();
      termRef.current = null;
      fitAddonRef.current = null;
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Build WebSocket URL
  const wsUrl = sessionId && isActive
    ? (() => {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const host = window.location.host;
        const apiKey = localStorage.getItem('swoops_api_key') || '';
        return `${protocol}//${host}/api/v1/ws/sessions/${sessionId}/output?token=${encodeURIComponent(apiKey)}`;
      })()
    : '';

  // WebSocket streaming with automatic reconnection
  const { state: wsState, retryCount, maxRetriesReached, reconnect } = useReconnectingWebSocket({
    url: wsUrl,
    enabled: !!sessionId && !!isActive && wsUrl !== '',
    maxRetries: 0, // Infinite retries
    initialBackoff: 1000, // Start at 1 second
    maxBackoff: 30000, // Max 30 seconds
    onMessage: (event) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'output' && msg.data) {
          writeOutput(msg.data);
        }
      } catch (error) {
        console.error('Failed to parse WebSocket message:', error);
      }
    },
    onError: (error) => {
      console.error('WebSocket error:', error);
    },
  });

  // Get connection status message
  const getConnectionStatus = () => {
    if (!sessionId || !isActive) return null;

    if (wsState === 'connecting' && retryCount === 0) {
      return { text: 'Connecting...', style: 'text-yellow-600 bg-yellow-50' };
    }

    if (wsState === 'connecting' && retryCount > 0) {
      return {
        text: `Reconnecting (attempt ${retryCount})...`,
        style: 'text-yellow-600 bg-yellow-50'
      };
    }

    if (wsState === 'closed' && !maxRetriesReached) {
      return {
        text: `Disconnected - reconnecting in ${Math.min(1000 * Math.pow(2, retryCount), 30000) / 1000}s...`,
        style: 'text-red-600 bg-red-50'
      };
    }

    if (maxRetriesReached) {
      return {
        text: 'Connection failed',
        style: 'text-red-600 bg-red-50',
        showRetry: true
      };
    }

    return null;
  };

  const connectionStatus = getConnectionStatus();

  return (
    <div className="relative w-full">
      {connectionStatus && (
        <div className={`absolute top-2 right-2 z-10 px-3 py-1.5 rounded-md text-sm font-medium flex items-center gap-2 ${connectionStatus.style} shadow-sm border border-current/20`}>
          {wsState === 'connecting' && (
            <svg className="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
          )}
          <span>{connectionStatus.text}</span>
          {connectionStatus.showRetry && (
            <button
              onClick={reconnect}
              className="ml-2 px-2 py-0.5 bg-white/50 hover:bg-white/80 rounded text-xs font-semibold transition-colors"
            >
              Retry
            </button>
          )}
        </div>
      )}
      <div
        ref={terminalRef}
        className="w-full"
        style={{ minHeight: '300px', maxHeight: '600px' }}
      />
    </div>
  );
}
