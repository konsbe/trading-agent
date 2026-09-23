import { render, screen, fireEvent } from '@testing-library/react';
import DialogModal from './DialogModal';

describe('DialogModal', () => {
  const mockCancel = jest.fn();
  const mockSubmit = jest.fn();

  const defaultProps = {
    isOpen: true,
    dialogTitle: 'Confirm Action',
    dialogBody: 'Are you sure you want to proceed?',
    cancelButton: {
      onSubmit: mockCancel,
      content: 'Cancel',
    },
    submitButton: {
      onSubmit: mockSubmit,
      content: 'Log Out',
      variant: 'danger' as const,
    }
  };

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('renders dialog with title, body, and buttons', () => {
    render(<DialogModal {...defaultProps} />);

    expect(screen.getByRole('dialog', { name: 'Confirm Action' })).toBeInTheDocument();
    expect(screen.getByText('Are you sure you want to proceed?')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /log out/i })).toBeInTheDocument();
  });

  it('calls appropriate handlers when buttons are clicked', () => {
    render(<DialogModal {...defaultProps} />);

    fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(mockCancel).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: /log out/i }));
    expect(mockSubmit).toHaveBeenCalledTimes(1);
  });

  it('applies button variants, defaulting cancel to secondary', () => {
    render(<DialogModal {...defaultProps} />);

    expect(screen.getByRole('button', { name: /log out/i })).toHaveClass('ta-button--danger');
    expect(screen.getByRole('button', { name: /cancel/i })).toHaveClass('ta-button--secondary');
  });

  it('defaults the submit button to primary and focuses it', () => {
    render(<DialogModal {...defaultProps} submitButton={{ content: 'Yes', onSubmit: mockSubmit }} />);

    const submit = screen.getByRole('button', { name: 'Yes' });
    expect(submit).toHaveClass('ta-button--primary');
    expect(submit).toHaveFocus();
  });

  it('does not close on overlay click', () => {
    render(<DialogModal {...defaultProps} />);

    fireEvent.click(screen.getByTestId('dialog-modal-overlay'));

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(mockCancel).not.toHaveBeenCalled();
  });

  it('does not render dialog when isOpen is false', () => {
    render(<DialogModal {...defaultProps} isOpen={false} />);
    expect(screen.queryByText('Confirm Action')).not.toBeInTheDocument();
  });
});
