import { useState, useEffect } from 'react';
import { Github, Eye, EyeOff, Check, X } from 'lucide-react';
import { githubApi } from '../api/github';

export function SettingsPage() {
  const [githubToken, setGithubToken] = useState('');
  const [showToken, setShowToken] = useState(false);
  const [saving, setSaving] = useState(false);
  const [verifying, setVerifying] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [isConfigured, setIsConfigured] = useState(false);

  useEffect(() => {
    // Check if GitHub token is configured by trying to list repos
    setVerifying(true);
    githubApi.listRepos()
      .then(() => {
        setIsConfigured(true);
        setVerifying(false);
      })
      .catch(() => {
        setIsConfigured(false);
        setVerifying(false);
      });
  }, []);

  const handleSaveToken = async () => {
    if (!githubToken.trim()) {
      setError('Please enter a GitHub token');
      return;
    }

    setSaving(true);
    setError('');
    setSuccess('');

    try {
      await githubApi.updateToken(githubToken);
      setSuccess('GitHub token saved successfully!');
      setIsConfigured(true);
      setGithubToken('');

      // Clear success message after 3 seconds
      setTimeout(() => setSuccess(''), 3000);
    } catch (err) {
      setError((err as Error).message || 'Failed to save GitHub token. Please check the token is valid.');
    } finally {
      setSaving(false);
    }
  };

  const handleRemoveToken = async () => {
    if (!confirm('Are you sure you want to remove your GitHub token? This will disable GitHub integration features.')) {
      return;
    }

    setSaving(true);
    setError('');
    setSuccess('');

    try {
      await githubApi.updateToken('');
      setSuccess('GitHub token removed successfully.');
      setIsConfigured(false);

      // Clear success message after 3 seconds
      setTimeout(() => setSuccess(''), 3000);
    } catch (err) {
      setError((err as Error).message || 'Failed to remove GitHub token.');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold mb-2">Settings</h1>
        <p className="text-sm text-gray-400">Manage your account and integration settings</p>
      </div>

      {/* GitHub Integration Section */}
      <div className="bg-gray-900 border border-gray-800 rounded-lg p-6 mb-6">
        <div className="flex items-start gap-4 mb-6">
          <div className="p-2 bg-gray-800 rounded-lg">
            <Github size={24} className="text-gray-400" />
          </div>
          <div className="flex-1">
            <h2 className="text-lg font-semibold mb-1">GitHub Integration</h2>
            <p className="text-sm text-gray-400">
              Connect your GitHub account to clone repositories, create new repos, and manage your projects directly from Swoops.
            </p>
          </div>
          <div className="flex items-center gap-2">
            {verifying ? (
              <span className="text-xs text-gray-500">Verifying...</span>
            ) : isConfigured ? (
              <span className="flex items-center gap-1.5 text-xs text-green-400">
                <Check size={14} />
                Connected
              </span>
            ) : (
              <span className="flex items-center gap-1.5 text-xs text-gray-500">
                <X size={14} />
                Not configured
              </span>
            )}
          </div>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">
              Personal Access Token
            </label>
            <div className="relative">
              <input
                type={showToken ? 'text' : 'password'}
                value={githubToken}
                onChange={(e) => setGithubToken(e.target.value)}
                placeholder="ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
                className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 pr-10 text-sm font-mono"
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    handleSaveToken();
                  }
                }}
              />
              <button
                type="button"
                onClick={() => setShowToken(!showToken)}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300"
              >
                {showToken ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
            <p className="text-xs text-gray-500 mt-2">
              Create a personal access token at{' '}
              <a
                href="https://github.com/settings/tokens"
                target="_blank"
                rel="noopener noreferrer"
                className="text-blue-400 hover:text-blue-300 underline"
              >
                github.com/settings/tokens
              </a>
              {' '}with <code className="bg-gray-800 px-1 py-0.5 rounded">repo</code> scope
            </p>
          </div>

          {error && (
            <div className="p-3 bg-red-900/20 border border-red-800 rounded text-sm text-red-400">
              {error}
            </div>
          )}

          {success && (
            <div className="p-3 bg-green-900/20 border border-green-800 rounded text-sm text-green-400">
              {success}
            </div>
          )}

          <div className="flex gap-3">
            <button
              onClick={handleSaveToken}
              disabled={saving || !githubToken.trim()}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-500 rounded text-sm disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {saving ? 'Saving...' : 'Save Token'}
            </button>
            {isConfigured && (
              <button
                onClick={handleRemoveToken}
                disabled={saving}
                className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-red-400 rounded text-sm disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Remove Token
              </button>
            )}
          </div>
        </div>

        {/* Token Permissions Guide */}
        <details className="mt-6 group">
          <summary className="cursor-pointer text-sm font-medium text-gray-400 hover:text-gray-300">
            What permissions does the token need?
          </summary>
          <div className="mt-3 p-4 bg-gray-950 rounded text-sm text-gray-400 space-y-2">
            <p>Your GitHub personal access token needs the following scopes:</p>
            <ul className="list-disc list-inside space-y-1 ml-2">
              <li><code className="bg-gray-800 px-1 py-0.5 rounded">repo</code> - Full control of private repositories (required for cloning, creating repos)</li>
            </ul>
            <p className="mt-3 text-xs text-gray-500">
              The token is stored securely and is only used to interact with GitHub API on your behalf.
            </p>
          </div>
        </details>
      </div>

      {/* Future sections can be added here */}
      {/* Example:
      <div className="bg-gray-900 border border-gray-800 rounded-lg p-6 mb-6">
        <h2 className="text-lg font-semibold mb-4">Notifications</h2>
        ...
      </div>
      */}
    </div>
  );
}
