import { useEffect, useState } from 'react';
import { api } from '../api/client';

interface SessionMessage {
  id: string;
  from_session_id: string;
  to_session_id: string;
  message_type: 'question' | 'info' | 'request' | 'response';
  subject: string;
  body: string;
  context?: Record<string, unknown>;
  status: 'sent' | 'read' | 'responded';
  read_at?: string;
  created_at: string;
}

interface SessionMessagesPanelProps {
  sessionId: string;
}

export function SessionMessagesPanel({ sessionId }: SessionMessagesPanelProps) {
  const [messages, setMessages] = useState<SessionMessage[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadMessages();
    const interval = setInterval(loadMessages, 10000); // Poll every 10 seconds
    return () => clearInterval(interval);
  }, [sessionId]);

  const loadMessages = async () => {
    try {
      const response = await api.get<SessionMessage[]>(`/sessions/${sessionId}/messages`);
      setMessages(response || []);
    } catch (error) {
      console.error('Failed to load messages:', error);
    } finally {
      setLoading(false);
    }
  };

  const markAsRead = async (messageId: string) => {
    try {
      await api.put(`/messages/${messageId}/read`, { session_id: sessionId });
      await loadMessages();
    } catch (error) {
      console.error('Failed to mark message as read:', error);
    }
  };

  const getMessageTypeColor = (type: string) => {
    switch (type) {
      case 'question': return 'text-purple-700 bg-purple-100';
      case 'info': return 'text-blue-700 bg-blue-100';
      case 'request': return 'text-orange-700 bg-orange-100';
      case 'response': return 'text-green-700 bg-green-100';
      default: return 'text-gray-700 bg-gray-100';
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'sent': return 'text-gray-600 bg-gray-50';
      case 'read': return 'text-blue-600 bg-blue-50';
      case 'responded': return 'text-green-600 bg-green-50';
      default: return 'text-gray-600 bg-gray-50';
    }
  };

  if (loading) {
    return <div className="text-sm text-gray-500">Loading messages...</div>;
  }

  if (messages.length === 0) {
    return (
      <div className="text-sm text-gray-500">
        No messages yet. Agents can coordinate using the coordinate_with_session MCP tool.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h3 className="text-sm font-medium text-gray-700">Session Messages</h3>
      <div className="space-y-2">
        {messages.map((message) => {
          const isIncoming = message.to_session_id === sessionId;
          const isUnread = isIncoming && message.status === 'sent';

          return (
            <div
              key={message.id}
              className={`border rounded-lg p-3 ${isUnread ? 'bg-blue-50 border-blue-200' : ''}`}
            >
              <div className="flex items-start justify-between mb-2">
                <div className="flex items-center gap-2">
                  <span className={`px-2 py-1 rounded text-xs font-medium ${getMessageTypeColor(message.message_type)}`}>
                    {message.message_type}
                  </span>
                  <span className={`px-2 py-1 rounded text-xs font-medium ${getStatusColor(message.status)}`}>
                    {message.status}
                  </span>
                  <span className="text-xs text-gray-600">
                    {isIncoming ? 'From:' : 'To:'} Session {isIncoming ? message.from_session_id.slice(0, 8) : message.to_session_id.slice(0, 8)}
                  </span>
                </div>
                <span className="text-xs text-gray-500">
                  {new Date(message.created_at).toLocaleString()}
                </span>
              </div>

              <h4 className="text-sm font-medium text-gray-900 mb-1">{message.subject}</h4>
              <p className="text-sm text-gray-700 whitespace-pre-wrap">{message.body}</p>

              {message.context && Object.keys(message.context).length > 0 && (
                <details className="mt-2">
                  <summary className="text-xs text-gray-500 cursor-pointer">View context</summary>
                  <pre className="mt-1 text-xs text-gray-600 bg-gray-50 p-2 rounded overflow-x-auto">
                    {JSON.stringify(message.context, null, 2)}
                  </pre>
                </details>
              )}

              {isUnread && (
                <button
                  onClick={() => markAsRead(message.id)}
                  className="mt-2 text-xs text-blue-600 hover:text-blue-700 underline"
                >
                  Mark as read
                </button>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
