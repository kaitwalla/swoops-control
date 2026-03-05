import { useEffect, useState } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '../stores/authStore';

interface ProtectedRouteProps {
  children: React.ReactNode;
}

export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const { user, token, fetchCurrentUser, loading, isInitialized } = useAuthStore();
  const location = useLocation();
  const [authTimeout, setAuthTimeout] = useState(false);

  useEffect(() => {
    // If we have a token but no user, try to fetch the current user
    if (token && !user && !loading && !isInitialized) {
      fetchCurrentUser();

      // Set a timeout to prevent infinite loading
      const timeout = setTimeout(() => {
        setAuthTimeout(true);
      }, 5000); // 5 second timeout

      return () => clearTimeout(timeout);
    }
  }, [token, user, loading, isInitialized, fetchCurrentUser]);

  // If no token, redirect to login
  if (!token) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  // If authentication timed out or fetching user failed (after initialization), redirect to login
  if (authTimeout || (token && !user && isInitialized)) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  // Show loading state while checking authentication (but only briefly)
  if (loading || (token && !isInitialized)) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-950">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto"></div>
          <p className="mt-4 text-gray-400">Loading...</p>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
