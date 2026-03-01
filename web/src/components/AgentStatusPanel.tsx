import { useCallback, useEffect, useState } from 'react';
import { api } from '../api/client';

interface AgentStatusUpdate {
  id: string;
  session_id: string;
  status_type: 'working' | 'idle' | 'blocked' | 'completed' | 'error';
  message: string;
  details?: Record<string, unknown>;
  created_at: string;
}

interface AgentStatusPanelProps {
  sessionId: string;
}

export function AgentStatusPanel({ sessionId }: AgentStatusPanelProps) {
  const [statusUpdates, setStatusUpdates] = useState<AgentStatusUpdate[]>([]);
  const [loading, setLoading] = useState(true);

  const loadStatusUpdates = useCallback(async () => {
    try {
      const response = await api.get<AgentStatusUpdate[]>(`/sessions/${sessionId}/status`);
      setStatusUpdates(response);
    } catch (error) {
      console.error('Failed to load status updates:', error);
    } finally {
      setLoading(false);
    }
  }, [sessionId]);

  useEffect(() => {
    loadStatusUpdates();
    const interval = setInterval(loadStatusUpdates, 5000); // Poll every 5 seconds
    return () => clearInterval(interval);
  }, [loadStatusUpdates]);

  const getStatusColor = (type: string) => {
    switch (type) {
      case 'working': return 'text-blue-600 bg-blue-50';
      case 'idle': return 'text-gray-600 bg-gray-50';
      case 'blocked': return 'text-yellow-600 bg-yellow-50';
      case 'completed': return 'text-green-600 bg-green-50';
      case 'error': return 'text-red-600 bg-red-50';
      default: return 'text-gray-600 bg-gray-50';
    }
  };

  if (loading) {
    return <div className="text-sm text-gray-500">Loading status...</div>;
  }

  if (statusUpdates.length === 0) {
    return (
      <div className="text-sm text-gray-500">
        No status updates yet. The agent will report status using the report_status MCP tool.
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <h3 className="text-sm font-medium text-gray-700">Agent Status Updates</h3>
      <div className="space-y-2 max-h-96 overflow-y-auto">
        {statusUpdates.map((update) => (
          <div key={update.id} className="border rounded-lg p-3">
            <div className="flex items-start justify-between">
              <div className="flex-1">
                <div className="flex items-center gap-2 mb-1">
                  <span className={`px-2 py-1 rounded text-xs font-medium ${getStatusColor(update.status_type)}`}>
                    {update.status_type}
                  </span>
                  <span className="text-xs text-gray-500">
                    {new Date(update.created_at).toLocaleString()}
                  </span>
                </div>
                <p className="text-sm text-gray-900">{update.message}</p>
                {update.details && Object.keys(update.details).length > 0 && (
                  <details className="mt-2">
                    <summary className="text-xs text-gray-500 cursor-pointer">View details</summary>
                    <pre className="mt-1 text-xs text-gray-600 bg-gray-50 p-2 rounded overflow-x-auto">
                      {JSON.stringify(update.details, null, 2)}
                    </pre>
                  </details>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
