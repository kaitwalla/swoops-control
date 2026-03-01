import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CreateHostDialog } from './CreateHostDialog';

describe('CreateHostDialog', () => {
  const defaultProps = {
    open: true,
    onClose: vi.fn(),
    onSubmit: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should not render when open is false', () => {
    const { container } = render(
      <CreateHostDialog {...defaultProps} open={false} />
    );
    expect(container.firstChild).toBeNull();
  });

  it('should render dialog when open is true', () => {
    render(<CreateHostDialog {...defaultProps} />);
    expect(screen.getByRole('heading', { name: 'Add Host' })).toBeInTheDocument();
  });

  it('should render all form fields with default values', () => {
    render(<CreateHostDialog {...defaultProps} />);

    expect(screen.getByPlaceholderText('gpu-box-1')).toHaveValue('');
    expect(screen.getByPlaceholderText('10.0.1.50')).toHaveValue('');
    expect(screen.getByPlaceholderText('deploy')).toHaveValue('');
    expect(screen.getByDisplayValue('22')).toBeInTheDocument();
    expect(screen.getByDisplayValue('10')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('/etc/swoops/keys/host.pem')).toHaveValue('');
    expect(screen.getByDisplayValue('/opt/swoops/repo')).toBeInTheDocument();
    expect(screen.getByDisplayValue('/opt/swoops/worktrees')).toBeInTheDocument();
  });

  it('should update form fields when user types', async () => {
    const user = userEvent.setup();
    render(<CreateHostDialog {...defaultProps} />);

    const nameInput = screen.getByPlaceholderText('gpu-box-1');
    await user.type(nameInput, 'My Host');

    expect(nameInput).toHaveValue('My Host');
  });

  it('should call onClose when close button is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<CreateHostDialog {...defaultProps} onClose={onClose} />);

    const closeButtons = screen.getAllByRole('button');
    // Find the X button (first button)
    const xButton = closeButtons[0];
    await user.click(xButton);

    expect(onClose).toHaveBeenCalled();
  });

  it('should call onClose when Cancel button is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<CreateHostDialog {...defaultProps} onClose={onClose} />);

    const cancelButton = screen.getByText('Cancel');
    await user.click(cancelButton);

    expect(onClose).toHaveBeenCalled();
  });

  it('should call onSubmit with form data when submitted', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<CreateHostDialog {...defaultProps} onSubmit={onSubmit} />);

    // Fill in required fields
    await user.type(screen.getByPlaceholderText('gpu-box-1'), 'Test Host');
    await user.type(screen.getByPlaceholderText('10.0.1.50'), 'test.example.com');
    await user.type(screen.getByPlaceholderText('deploy'), 'testuser');
    await user.type(screen.getByPlaceholderText('/etc/swoops/keys/host.pem'), '/path/to/key');

    const submitButton = screen.getByRole('button', { name: 'Add Host' });
    await user.click(submitButton);

    expect(onSubmit).toHaveBeenCalledWith({
      name: 'Test Host',
      hostname: 'test.example.com',
      ssh_port: 22,
      ssh_user: 'testuser',
      ssh_key_path: '/path/to/key',
      max_sessions: 10,
      labels: {},
      base_repo_path: '/opt/swoops/repo',
      worktree_root: '/opt/swoops/worktrees',
    });
  });

  it('should close dialog after successful submit', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<CreateHostDialog {...defaultProps} onClose={onClose} onSubmit={onSubmit} />);

    await user.type(screen.getByPlaceholderText('gpu-box-1'), 'Test Host');
    await user.type(screen.getByPlaceholderText('10.0.1.50'), 'test.example.com');
    await user.type(screen.getByPlaceholderText('deploy'), 'testuser');
    await user.type(screen.getByPlaceholderText('/etc/swoops/keys/host.pem'), '/path/to/key');

    const submitButton = screen.getByRole('button', { name: 'Add Host' });
    await user.click(submitButton);

    // Wait for async operations
    await vi.waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('should show loading state while submitting', async () => {
    const user = userEvent.setup();
    let resolveSubmit: () => void;
    const onSubmit = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveSubmit = resolve;
        })
    );
    render(<CreateHostDialog {...defaultProps} onSubmit={onSubmit} />);

    await user.type(screen.getByPlaceholderText('gpu-box-1'), 'Test Host');
    await user.type(screen.getByPlaceholderText('10.0.1.50'), 'test.example.com');
    await user.type(screen.getByPlaceholderText('deploy'), 'testuser');
    await user.type(screen.getByPlaceholderText('/etc/swoops/keys/host.pem'), '/path/to/key');

    const submitButton = screen.getByRole('button', { name: 'Add Host' });
    await user.click(submitButton);

    // Should show loading text
    const loadingButton = await screen.findByRole('button', { name: 'Adding...' });
    expect(loadingButton).toBeDisabled();

    // Resolve the promise
    resolveSubmit!();
  });

  it('should display error message when submit fails', async () => {
    const user = userEvent.setup();
    const errorMessage = 'Failed to create host';
    const onSubmit = vi.fn().mockRejectedValue(new Error(errorMessage));
    render(<CreateHostDialog {...defaultProps} onSubmit={onSubmit} />);

    await user.type(screen.getByPlaceholderText('gpu-box-1'), 'Test Host');
    await user.type(screen.getByPlaceholderText('10.0.1.50'), 'test.example.com');
    await user.type(screen.getByPlaceholderText('deploy'), 'testuser');
    await user.type(screen.getByPlaceholderText('/etc/swoops/keys/host.pem'), '/path/to/key');

    const submitButton = screen.getByRole('button', { name: 'Add Host' });
    await user.click(submitButton);

    // Wait for error to appear
    await vi.waitFor(() => {
      expect(screen.getByText(errorMessage)).toBeInTheDocument();
    });
  });

  it('should update numeric fields correctly', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<CreateHostDialog {...defaultProps} onSubmit={onSubmit} />);

    await user.type(screen.getByPlaceholderText('gpu-box-1'), 'Test Host');
    await user.type(screen.getByPlaceholderText('10.0.1.50'), 'test.example.com');
    await user.type(screen.getByPlaceholderText('deploy'), 'testuser');
    await user.type(screen.getByPlaceholderText('/etc/swoops/keys/host.pem'), '/path/to/key');

    // Update SSH port - use triple click to select all then type
    const sshPortInput = screen.getByDisplayValue('22');
    await user.tripleClick(sshPortInput);
    await user.keyboard('2222');

    // Update max sessions - use triple click to select all then type
    const maxSessionsInput = screen.getByDisplayValue('10');
    await user.tripleClick(maxSessionsInput);
    await user.keyboard('5');

    const submitButton = screen.getByRole('button', { name: 'Add Host' });
    await user.click(submitButton);

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        ssh_port: 2222,
        max_sessions: 5,
      })
    );
  });
});
