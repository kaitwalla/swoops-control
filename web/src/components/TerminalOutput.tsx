import { useEffect, useRef, useCallback } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebLinksAddon } from '@xterm/addon-web-links';
import '@xterm/xterm/css/xterm.css';

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
  const wsRef = useRef<WebSocket | null>(null);
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

  // WebSocket streaming
  useEffect(() => {
    if (!sessionId || !isActive) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const apiKey = localStorage.getItem('swoops_api_key') || '';
    const ws = new WebSocket(
      `${protocol}//${host}/api/v1/ws/sessions/${sessionId}/output?token=${encodeURIComponent(apiKey)}`
    );

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'output' && msg.data) {
          writeOutput(msg.data);
        }
      } catch {
        // ignore parse errors
      }
    };

    ws.onerror = () => {
      // Fall back to polling if WebSocket fails
    };

    wsRef.current = ws;

    return () => {
      ws.close();
      wsRef.current = null;
    };
  }, [sessionId, isActive, writeOutput]);

  // Polling fallback: refresh output every 2s when active but no WebSocket
  useEffect(() => {
    if (!sessionId || !isActive) return;

    const poll = setInterval(async () => {
      // Only poll if WebSocket is not connected
      if (wsRef.current?.readyState === WebSocket.OPEN) return;

      try {
        const resp = await fetch(`/api/v1/sessions/${sessionId}/output`, {
          headers: {
            Authorization: `Bearer ${localStorage.getItem('swoops_api_key') || ''}`,
          },
        });
        if (resp.ok) {
          const data = await resp.json();
          if (data.output) {
            writeOutput(data.output);
          }
        }
      } catch {
        // ignore polling errors
      }
    }, 2000);

    return () => clearInterval(poll);
  }, [sessionId, isActive, writeOutput]);

  return (
    <div
      ref={terminalRef}
      className="w-full"
      style={{ minHeight: '300px', maxHeight: '600px' }}
    />
  );
}
