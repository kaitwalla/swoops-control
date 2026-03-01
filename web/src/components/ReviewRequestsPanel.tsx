import { useEffect, useState } from 'react';
import { api } from '../api/client';

interface ReviewRequest {
  id: string;
  session_id: string;
  request_type: 'code' | 'architecture' | 'security' | 'performance';
  title: string;
  description: string;
  file_paths?: string[];
  diff?: string;
  status: 'pending' | 'in_review' | 'approved' | 'changes_requested' | 'rejected';
  reviewer_notes?: string;
  reviewed_at?: string;
  created_at: string;
  updated_at: string;
}

interface ReviewRequestsPanelProps {
  sessionId: string;
}

export function ReviewRequestsPanel({ sessionId }: ReviewRequestsPanelProps) {
  const [reviews, setReviews] = useState<ReviewRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedReview, setSelectedReview] = useState<ReviewRequest | null>(null);
  const [reviewNotes, setReviewNotes] = useState('');

  useEffect(() => {
    loadReviews();
  }, [sessionId]);

  const loadReviews = async () => {
    try {
      const response = await api.get<ReviewRequest[]>(`/reviews?session_id=${sessionId}`);
      setReviews(response || []);
    } catch (error) {
      console.error('Failed to load reviews:', error);
    } finally {
      setLoading(false);
    }
  };

  const updateReview = async (reviewId: string, status: string) => {
    try {
      await api.put(`/reviews/${reviewId}`, {
        status,
        notes: reviewNotes
      });
      setSelectedReview(null);
      setReviewNotes('');
      await loadReviews();
    } catch (error) {
      console.error('Failed to update review:', error);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'pending': return 'text-yellow-600 bg-yellow-50';
      case 'in_review': return 'text-blue-600 bg-blue-50';
      case 'approved': return 'text-green-600 bg-green-50';
      case 'changes_requested': return 'text-orange-600 bg-orange-50';
      case 'rejected': return 'text-red-600 bg-red-50';
      default: return 'text-gray-600 bg-gray-50';
    }
  };

  const getTypeColor = (type: string) => {
    switch (type) {
      case 'code': return 'text-blue-700 bg-blue-100';
      case 'architecture': return 'text-purple-700 bg-purple-100';
      case 'security': return 'text-red-700 bg-red-100';
      case 'performance': return 'text-green-700 bg-green-100';
      default: return 'text-gray-700 bg-gray-100';
    }
  };

  if (loading) {
    return <div className="text-sm text-gray-500">Loading reviews...</div>;
  }

  if (reviews.length === 0) {
    return (
      <div className="text-sm text-gray-500">
        No review requests yet. The agent can request reviews using the request_review MCP tool.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h3 className="text-sm font-medium text-gray-700">Review Requests</h3>

      {selectedReview && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-lg p-6 max-w-2xl w-full max-h-[90vh] overflow-y-auto">
            <h3 className="text-lg font-semibold mb-4">Review: {selectedReview.title}</h3>

            <div className="space-y-3 mb-4">
              <div className="flex gap-2">
                <span className={`px-2 py-1 rounded text-xs font-medium ${getTypeColor(selectedReview.request_type)}`}>
                  {selectedReview.request_type}
                </span>
                <span className={`px-2 py-1 rounded text-xs font-medium ${getStatusColor(selectedReview.status)}`}>
                  {selectedReview.status}
                </span>
              </div>

              <div>
                <h4 className="text-sm font-medium text-gray-700 mb-1">Description</h4>
                <p className="text-sm text-gray-900 whitespace-pre-wrap">{selectedReview.description}</p>
              </div>

              {selectedReview.file_paths && selectedReview.file_paths.length > 0 && (
                <div>
                  <h4 className="text-sm font-medium text-gray-700 mb-1">Files</h4>
                  <ul className="text-sm text-gray-600 list-disc list-inside">
                    {selectedReview.file_paths.map((path, i) => (
                      <li key={i}>{path}</li>
                    ))}
                  </ul>
                </div>
              )}

              {selectedReview.diff && (
                <div>
                  <h4 className="text-sm font-medium text-gray-700 mb-1">Diff</h4>
                  <pre className="text-xs bg-gray-50 p-3 rounded overflow-x-auto">{selectedReview.diff}</pre>
                </div>
              )}

              {selectedReview.status !== 'pending' && selectedReview.reviewer_notes && (
                <div>
                  <h4 className="text-sm font-medium text-gray-700 mb-1">Reviewer Notes</h4>
                  <p className="text-sm text-gray-900">{selectedReview.reviewer_notes}</p>
                </div>
              )}
            </div>

            {selectedReview.status === 'pending' && (
              <div className="space-y-3">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Review Notes</label>
                  <textarea
                    value={reviewNotes}
                    onChange={(e) => setReviewNotes(e.target.value)}
                    className="w-full border rounded px-3 py-2 text-sm h-24"
                    placeholder="Optional notes for the agent..."
                  />
                </div>

                <div className="flex gap-2">
                  <button
                    onClick={() => updateReview(selectedReview.id, 'approved')}
                    className="flex-1 bg-green-600 text-white px-4 py-2 rounded hover:bg-green-700 text-sm"
                  >
                    Approve
                  </button>
                  <button
                    onClick={() => updateReview(selectedReview.id, 'changes_requested')}
                    className="flex-1 bg-orange-600 text-white px-4 py-2 rounded hover:bg-orange-700 text-sm"
                  >
                    Request Changes
                  </button>
                  <button
                    onClick={() => updateReview(selectedReview.id, 'rejected')}
                    className="flex-1 bg-red-600 text-white px-4 py-2 rounded hover:bg-red-700 text-sm"
                  >
                    Reject
                  </button>
                </div>
              </div>
            )}

            <button
              onClick={() => setSelectedReview(null)}
              className="mt-4 w-full border border-gray-300 px-4 py-2 rounded hover:bg-gray-50 text-sm"
            >
              Close
            </button>
          </div>
        </div>
      )}

      <div className="space-y-2">
        {reviews.map((review) => (
          <div key={review.id} className="border rounded-lg p-3 hover:bg-gray-50 cursor-pointer" onClick={() => setSelectedReview(review)}>
            <div className="flex items-start justify-between mb-2">
              <div className="flex items-center gap-2">
                <span className={`px-2 py-1 rounded text-xs font-medium ${getTypeColor(review.request_type)}`}>
                  {review.request_type}
                </span>
                <span className={`px-2 py-1 rounded text-xs font-medium ${getStatusColor(review.status)}`}>
                  {review.status}
                </span>
              </div>
              <span className="text-xs text-gray-500">
                {new Date(review.created_at).toLocaleString()}
              </span>
            </div>
            <h4 className="text-sm font-medium text-gray-900 mb-1">{review.title}</h4>
            <p className="text-sm text-gray-600 line-clamp-2">{review.description}</p>
          </div>
        ))}
      </div>
    </div>
  );
}
