import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatusBadge } from './StatusBadge';

describe('StatusBadge', () => {
  it('should render the status text', () => {
    render(<StatusBadge status="online" />);
    expect(screen.getByText('online')).toBeInTheDocument();
  });

  it('should apply correct color for online status', () => {
    const { container } = render(<StatusBadge status="online" />);
    const badge = container.querySelector('span');
    expect(badge).toHaveClass('bg-green-500/20', 'text-green-400');
  });

  it('should apply correct color for offline status', () => {
    const { container } = render(<StatusBadge status="offline" />);
    const badge = container.querySelector('span');
    expect(badge).toHaveClass('bg-gray-500/20', 'text-gray-400');
  });

  it('should apply correct color for degraded status', () => {
    const { container } = render(<StatusBadge status="degraded" />);
    const badge = container.querySelector('span');
    expect(badge).toHaveClass('bg-yellow-500/20', 'text-yellow-400');
  });

  it('should apply correct color for provisioning status', () => {
    const { container } = render(<StatusBadge status="provisioning" />);
    const badge = container.querySelector('span');
    expect(badge).toHaveClass('bg-blue-500/20', 'text-blue-400');
  });

  it('should apply correct color for running status', () => {
    const { container } = render(<StatusBadge status="running" />);
    const badge = container.querySelector('span');
    expect(badge).toHaveClass('bg-green-500/20', 'text-green-400');
  });

  it('should apply correct color for stopped status', () => {
    const { container } = render(<StatusBadge status="stopped" />);
    const badge = container.querySelector('span');
    expect(badge).toHaveClass('bg-gray-500/20', 'text-gray-400');
  });

  it('should apply correct color for failed status', () => {
    const { container } = render(<StatusBadge status="failed" />);
    const badge = container.querySelector('span');
    expect(badge).toHaveClass('bg-red-500/20', 'text-red-400');
  });

  it('should apply default color for unknown status', () => {
    const { container } = render(<StatusBadge status="unknown-status" />);
    const badge = container.querySelector('span');
    expect(badge).toHaveClass('bg-gray-500/20', 'text-gray-400');
  });
});
