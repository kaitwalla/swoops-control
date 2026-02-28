import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { Layout } from './components/Layout';
import { Dashboard } from './pages/Dashboard';
import { HostsPage } from './pages/HostsPage';
import { HostDetail } from './pages/HostDetail';
import { SessionsPage } from './pages/SessionsPage';
import { SessionDetail } from './pages/SessionDetail';
import { PluginsPage } from './pages/PluginsPage';
import { TemplatesPage } from './pages/TemplatesPage';

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/hosts" element={<HostsPage />} />
          <Route path="/hosts/:id" element={<HostDetail />} />
          <Route path="/sessions" element={<SessionsPage />} />
          <Route path="/sessions/:id" element={<SessionDetail />} />
          <Route path="/plugins" element={<PluginsPage />} />
          <Route path="/templates" element={<TemplatesPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
