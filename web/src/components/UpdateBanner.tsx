import { useEffect, useState } from 'react';
import { X, AlertCircle, ExternalLink } from 'lucide-react';
import { versionApi, type VersionInfo } from '../api/version';

export function UpdateBanner() {
  const [versionInfo, setVersionInfo] = useState<VersionInfo | null>(null);
  const [dismissed, setDismissed] = useState(false);

  useEffect(() => {
    // Check for updates on mount
    versionApi.get()
      .then(setVersionInfo)
      .catch((err) => console.error('Failed to fetch version info:', err));

    // Check periodically (every hour)
    const interval = setInterval(() => {
      versionApi.get()
        .then(setVersionInfo)
        .catch(() => {}); // Silently fail on background checks
    }, 60 * 60 * 1000);

    return () => clearInterval(interval);
  }, []);

  // Don't show banner if dismissed or no update available
  if (dismissed || !versionInfo?.update_available) {
    return null;
  }

  return (
    <div className="bg-blue-600 text-white px-4 py-3 flex items-center justify-between">
      <div className="flex items-center gap-3">
        <AlertCircle size={20} />
        <div className="text-sm">
          <span className="font-medium">Update available:</span>{' '}
          v{versionInfo.version} → v{versionInfo.latest_version}
          {versionInfo.update_url && (
            <a
              href={versionInfo.update_url}
              target="_blank"
              rel="noopener noreferrer"
              className="ml-2 underline inline-flex items-center gap-1 hover:text-blue-100"
            >
              View release <ExternalLink size={14} />
            </a>
          )}
        </div>
      </div>
      <button
        onClick={() => setDismissed(true)}
        className="text-white/80 hover:text-white"
        aria-label="Dismiss update notification"
      >
        <X size={18} />
      </button>
    </div>
  );
}
