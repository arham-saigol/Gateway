import { useState } from 'preact/hooks';
import type { ComponentChildren } from 'preact';
import { IconCopy, IconCheck, IconX, IconInfo, IconSearch } from './Icons';

interface BadgeProps {
  variant?: 'success' | 'danger' | 'warning' | 'info' | 'muted' | 'neutral';
  withDot?: boolean;
  children: ComponentChildren;
  class?: string;
}

export function Badge({ variant = 'neutral', withDot = true, children, class: className = '' }: BadgeProps) {
  return (
    <span class={`badge badge-${variant} ${className}`}>
      {withDot && <span class="badge-dot" />}
      {children}
    </span>
  );
}

interface CopyButtonProps {
  text: string;
  label?: string;
  size?: 'xs' | 'sm' | 'md';
}

export function CopyButton({ text, label, size = 'sm' }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  function handleCopy(e: MouseEvent) {
    e.stopPropagation();
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <button
      type="button"
      class={`btn btn-secondary btn-${size}`}
      onClick={handleCopy}
      title="Copy to clipboard"
      style={{ display: 'inline-flex', alignItems: 'center', gap: '5px' }}
    >
      {copied ? <IconCheck size={13} class="text-success" /> : <IconCopy size={13} />}
      {label && <span>{copied ? 'Copied' : label}</span>}
    </button>
  );
}

interface CalloutProps {
  children: ComponentChildren;
  icon?: ComponentChildren;
}

export function Callout({ children, icon }: CalloutProps) {
  return (
    <div class="notion-callout">
      {icon || <IconInfo size={16} />}
      <div>{children}</div>
    </div>
  );
}

interface MetricCardProps {
  label: string;
  value: string | number;
  subtext?: string;
  valueColor?: string;
  badge?: ComponentChildren;
}

export function MetricCard({ label, value, subtext, valueColor, badge }: MetricCardProps) {
  return (
    <div class="stat-card">
      <div class="stat-label">
        <span>{label}</span>
        {badge}
      </div>
      <div class="stat-value" style={valueColor ? { color: valueColor } : undefined}>
        {value}
      </div>
      {subtext && <div class="stat-sub">{subtext}</div>}
    </div>
  );
}

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  maxWidth?: string;
  children: ComponentChildren;
  footer?: ComponentChildren;
}

export function Modal({ isOpen, onClose, title, maxWidth = '520px', children, footer }: ModalProps) {
  if (!isOpen) return null;

  return (
    <div class="modal-overlay" onClick={onClose}>
      <div class="modal-content" style={{ maxWidth }} onClick={e => e.stopPropagation()}>
        <div class="modal-header">
          <div class="modal-title">{title}</div>
          <button class="btn btn-ghost btn-icon" onClick={onClose} aria-label="Close">
            <IconX size={15} />
          </button>
        </div>
        <div class="modal-body">
          {children}
        </div>
        {footer && (
          <div class="modal-footer">
            {footer}
          </div>
        )}
      </div>
    </div>
  );
}

interface SearchInputProps {
  value: string;
  onInput: (val: string) => void;
  placeholder?: string;
}

export function SearchInput({ value, onInput, placeholder = 'Search...' }: SearchInputProps) {
  return (
    <div class="search-input-wrapper">
      <IconSearch size={14} />
      <input
        type="text"
        class="form-input"
        placeholder={placeholder}
        value={value}
        onInput={(e: any) => onInput(e.target.value)}
      />
    </div>
  );
}

interface EmptyStateProps {
  icon?: ComponentChildren;
  title: string;
  description?: string;
  action?: ComponentChildren;
}

export function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  return (
    <div class="empty-state">
      {icon && <div class="empty-state-icon">{icon}</div>}
      <div class="empty-state-title">{title}</div>
      {description && <div class="empty-state-text">{description}</div>}
      {action && <div style={{ marginTop: '16px' }}>{action}</div>}
    </div>
  );
}
