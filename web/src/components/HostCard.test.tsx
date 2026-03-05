import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import userEvent from '@testing-library/user-event';
import { HostCard } from './HostCard';
import type { Host } from '../types/host';

const mockHost: Host = {
  id: 'host-1',
  name: 'Test Host',
  hostname: 'test.example.com',
  ssh_port: 22,
  ssh_user: 'testuser',
  ssh_key_path: '/path/to/key',
  os: 'linux',
  arch: 'x86_64',
  status: 'online',
  agent_version: '1.0.0',
  update_available: false,
  labels: { env: 'test', region: 'us-west' },
  max_sessions: 5,
  base_repo_path: '/repos',
  worktree_root: '/worktrees',
  installed_plugins: [],
  installed_tools: [],
  last_heartbeat: '2024-01-01T00:00:00Z',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
};

// Helper to render with router
const renderWithRouter = (ui: React.ReactElement) => {
  return render(<BrowserRouter>{ui}</BrowserRouter>);
};

describe('HostCard', () => {
  it('should render host name and status', () => {
    const onDelete = vi.fn();
    const onUpdate = vi.fn();
    renderWithRouter(<HostCard host={mockHost} sessionCount={2} onDelete={onDelete} onUpdate={onUpdate} />);

    expect(screen.getByText('Test Host')).toBeInTheDocument();
    expect(screen.getByText('online')).toBeInTheDocument();
  });

  it('should render host connection details', () => {
    const onDelete = vi.fn();
    renderWithRouter(<HostCard host={mockHost} sessionCount={2} onDelete={onDelete} onUpdate={vi.fn()} />);

    expect(screen.getByText('test.example.com:22')).toBeInTheDocument();
    expect(screen.getByText('testuser@linux/x86_64')).toBeInTheDocument();
  });

  it('should render session count with singular form', () => {
    const onDelete = vi.fn();
    renderWithRouter(<HostCard host={mockHost} sessionCount={1} onDelete={onDelete} onUpdate={vi.fn()} />);

    expect(screen.getByText('1 session / 5 max')).toBeInTheDocument();
  });

  it('should render session count with plural form', () => {
    const onDelete = vi.fn();
    renderWithRouter(<HostCard host={mockHost} sessionCount={3} onDelete={onDelete} onUpdate={vi.fn()} />);

    expect(screen.getByText('3 sessions / 5 max')).toBeInTheDocument();
  });

  it('should render host labels', () => {
    const onDelete = vi.fn();
    renderWithRouter(<HostCard host={mockHost} sessionCount={2} onDelete={onDelete} onUpdate={vi.fn()} />);

    expect(screen.getByText('env=test')).toBeInTheDocument();
    expect(screen.getByText('region=us-west')).toBeInTheDocument();
  });

  it('should render without labels when labels object is empty', () => {
    const hostWithoutLabels = { ...mockHost, labels: {} };
    const onDelete = vi.fn();
    renderWithRouter(<HostCard host={hostWithoutLabels} sessionCount={2} onDelete={onDelete} onUpdate={vi.fn()} />);

    expect(screen.queryByText(/env=/)).not.toBeInTheDocument();
  });

  it('should link to host detail page', () => {
    const onDelete = vi.fn();
    const { container } = renderWithRouter(
      <HostCard host={mockHost} sessionCount={2} onDelete={onDelete} onUpdate={vi.fn()} />
    );

    const link = container.querySelector('a[href="/hosts/host-1"]');
    expect(link).toBeInTheDocument();
  });

  it('should call onDelete when delete button is clicked', async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    const { container } = renderWithRouter(
      <HostCard host={mockHost} sessionCount={2} onDelete={onDelete} onUpdate={vi.fn()} />
    );

    // Find delete button (has Trash2 icon)
    const deleteButton = container.querySelector('button');
    expect(deleteButton).toBeInTheDocument();

    await user.click(deleteButton!);

    expect(onDelete).toHaveBeenCalledWith('host-1');
    expect(onDelete).toHaveBeenCalledTimes(1);
  });

  it('should handle missing os and arch gracefully', () => {
    const hostWithoutOsArch = {
      ...mockHost,
      os: '',
      arch: '',
    };
    const onDelete = vi.fn();
    renderWithRouter(<HostCard host={hostWithoutOsArch} sessionCount={2} onDelete={onDelete} onUpdate={vi.fn()} />);

    expect(screen.getByText('testuser@unknown/unknown')).toBeInTheDocument();
  });
});
