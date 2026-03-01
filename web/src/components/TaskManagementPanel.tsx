import { useCallback, useEffect, useState } from 'react';
import { api } from '../api/client';

interface SessionTask {
  id: string;
  session_id: string;
  task_type: 'instruction' | 'fix' | 'review' | 'refactor' | 'test';
  priority: number;
  title: string;
  description: string;
  context?: Record<string, unknown>;
  status: 'pending' | 'retrieved' | 'completed' | 'failed';
  retrieved_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
}

interface TaskManagementPanelProps {
  sessionId: string;
}

export function TaskManagementPanel({ sessionId }: TaskManagementPanelProps) {
  const [tasks, setTasks] = useState<SessionTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [newTask, setNewTask] = useState<{
    task_type: SessionTask['task_type'];
    priority: number;
    title: string;
    description: string;
  }>({
    task_type: 'instruction',
    priority: 0,
    title: '',
    description: ''
  });

  const loadTasks = useCallback(async () => {
    try {
      const response = await api.get<SessionTask[]>(`/sessions/${sessionId}/tasks`);
      setTasks(response || []);
    } catch (error) {
      console.error('Failed to load tasks:', error);
    } finally {
      setLoading(false);
    }
  }, [sessionId]);

  useEffect(() => {
    loadTasks();
  }, [loadTasks]);

  const createTask = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newTask.title || !newTask.description) return;

    try {
      setSubmitting(true);
      await api.post(`/sessions/${sessionId}/tasks`, newTask);
      setNewTask({ task_type: 'instruction', priority: 0, title: '', description: '' });
      setShowCreateForm(false);
      await loadTasks();
    } catch (error) {
      console.error('Failed to create task:', error);
    } finally {
      setSubmitting(false);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'pending': return 'text-gray-600 bg-gray-50';
      case 'retrieved': return 'text-blue-600 bg-blue-50';
      case 'completed': return 'text-green-600 bg-green-50';
      case 'failed': return 'text-red-600 bg-red-50';
      default: return 'text-gray-600 bg-gray-50';
    }
  };

  const getTaskTypeColor = (type: string) => {
    switch (type) {
      case 'instruction': return 'text-blue-700 bg-blue-100';
      case 'fix': return 'text-red-700 bg-red-100';
      case 'review': return 'text-purple-700 bg-purple-100';
      case 'refactor': return 'text-yellow-700 bg-yellow-100';
      case 'test': return 'text-green-700 bg-green-100';
      default: return 'text-gray-700 bg-gray-100';
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <h3 className="text-sm font-medium text-gray-700">Tasks</h3>
        <button
          onClick={() => setShowCreateForm(!showCreateForm)}
          className="text-sm px-3 py-1 bg-blue-600 text-white rounded hover:bg-blue-700"
        >
          {showCreateForm ? 'Cancel' : 'New Task'}
        </button>
      </div>

      {showCreateForm && (
        <form onSubmit={createTask} className="border rounded-lg p-4 space-y-3 bg-gray-50">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Type</label>
            <select
              value={newTask.task_type}
              onChange={(e) => setNewTask({ ...newTask, task_type: e.target.value as SessionTask['task_type'] })}
              className="w-full border rounded px-3 py-2 text-sm"
            >
              <option value="instruction">Instruction</option>
              <option value="fix">Fix</option>
              <option value="review">Review</option>
              <option value="refactor">Refactor</option>
              <option value="test">Test</option>
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Priority</label>
            <input
              type="number"
              value={newTask.priority}
              onChange={(e) => setNewTask({ ...newTask, priority: parseInt(e.target.value) })}
              className="w-full border rounded px-3 py-2 text-sm"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Title</label>
            <input
              type="text"
              value={newTask.title}
              onChange={(e) => setNewTask({ ...newTask, title: e.target.value })}
              className="w-full border rounded px-3 py-2 text-sm"
              required
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
            <textarea
              value={newTask.description}
              onChange={(e) => setNewTask({ ...newTask, description: e.target.value })}
              className="w-full border rounded px-3 py-2 text-sm h-24"
              required
            />
          </div>
          <button
            type="submit"
            disabled={submitting}
            className="w-full bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700 text-sm disabled:opacity-50"
          >
            {submitting ? 'Creating...' : 'Create Task'}
          </button>
        </form>
      )}

      {loading ? (
        <div className="text-sm text-gray-500">Loading tasks...</div>
      ) : tasks.length === 0 ? (
        <div className="text-sm text-gray-500">
          No tasks yet. Create tasks to guide the agent's work.
        </div>
      ) : (
        <div className="space-y-2">
          {tasks.map((task) => (
            <div key={task.id} className="border rounded-lg p-3">
              <div className="flex items-start justify-between mb-2">
                <div className="flex items-center gap-2">
                  <span className={`px-2 py-1 rounded text-xs font-medium ${getTaskTypeColor(task.task_type)}`}>
                    {task.task_type}
                  </span>
                  <span className={`px-2 py-1 rounded text-xs font-medium ${getStatusColor(task.status)}`}>
                    {task.status}
                  </span>
                  {task.priority > 0 && (
                    <span className="text-xs text-gray-500">Priority: {task.priority}</span>
                  )}
                </div>
              </div>
              <h4 className="text-sm font-medium text-gray-900 mb-1">{task.title}</h4>
              <p className="text-sm text-gray-600 whitespace-pre-wrap">{task.description}</p>
              <div className="mt-2 text-xs text-gray-500">
                Created: {new Date(task.created_at).toLocaleString()}
                {task.retrieved_at && ` • Retrieved: ${new Date(task.retrieved_at).toLocaleString()}`}
                {task.completed_at && ` • Completed: ${new Date(task.completed_at).toLocaleString()}`}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
