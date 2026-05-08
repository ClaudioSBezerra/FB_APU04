import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ResetDatabaseDialog } from './ResetDatabaseDialog';

describe('ResetDatabaseDialog', () => {
  it('not rendered in DOM when open=false', () => {
    render(<ResetDatabaseDialog open={false} onOpenChange={() => {}} onConfirm={() => {}} />);
    expect(screen.queryByRole('alertdialog')).toBeNull();
  });

  it('renders title when open=true', () => {
    render(<ResetDatabaseDialog open={true} onOpenChange={() => {}} onConfirm={() => {}} />);
    expect(screen.getByText(/Zerar TODA a base/i)).toBeInTheDocument();
  });

  it('confirm button starts disabled', () => {
    render(<ResetDatabaseDialog open={true} onOpenChange={() => {}} onConfirm={() => {}} />);
    expect(screen.getByRole('button', { name: /Confirmar destruição/i })).toBeDisabled();
  });

  it('confirm button enables when exact token is typed', () => {
    render(<ResetDatabaseDialog open={true} onOpenChange={() => {}} onConfirm={() => {}} />);
    fireEvent.change(screen.getByLabelText('Token de confirmação'), {
      target: { value: 'DELETE-FB_APU04' },
    });
    expect(screen.getByRole('button', { name: /Confirmar destruição/i })).toBeEnabled();
  });

  it('confirm button stays disabled for case-mismatched token', () => {
    render(<ResetDatabaseDialog open={true} onOpenChange={() => {}} onConfirm={() => {}} />);
    fireEvent.change(screen.getByLabelText('Token de confirmação'), {
      target: { value: 'delete-fb_apu04' },
    });
    expect(screen.getByRole('button', { name: /Confirmar destruição/i })).toBeDisabled();
  });

  it('confirm button stays disabled with trailing space', () => {
    render(<ResetDatabaseDialog open={true} onOpenChange={() => {}} onConfirm={() => {}} />);
    fireEvent.change(screen.getByLabelText('Token de confirmação'), {
      target: { value: 'DELETE-FB_APU04 ' },
    });
    expect(screen.getByRole('button', { name: /Confirmar destruição/i })).toBeDisabled();
  });

  it('clicking Confirm calls onConfirm with body once', () => {
    const onConfirm = vi.fn();
    render(<ResetDatabaseDialog open={true} onOpenChange={() => {}} onConfirm={onConfirm} />);
    fireEvent.change(screen.getByLabelText('Token de confirmação'), {
      target: { value: 'DELETE-FB_APU04' },
    });
    fireEvent.click(screen.getByRole('button', { name: /Confirmar destruição/i }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onConfirm).toHaveBeenCalledWith({ confirmation: 'DELETE-FB_APU04' });
  });

  it('clicking Cancelar calls onOpenChange(false) and not onConfirm', () => {
    const onOpenChange = vi.fn();
    const onConfirm = vi.fn();
    render(<ResetDatabaseDialog open={true} onOpenChange={onOpenChange} onConfirm={onConfirm} />);
    fireEvent.click(screen.getByRole('button', { name: /Cancelar/i }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(onConfirm).not.toHaveBeenCalled();
  });
});
