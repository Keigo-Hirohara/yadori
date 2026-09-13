import type { ReactNode } from "react";

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";

export function Button({
  children,
  onClick,
  type = "button",
  variant = "primary",
  disabled,
  className = "",
}: {
  children: ReactNode;
  onClick?: () => void;
  type?: "button" | "submit";
  variant?: ButtonVariant;
  disabled?: boolean;
  className?: string;
}) {
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      className={`btn btn-${variant} ${className}`}
    >
      {children}
    </button>
  );
}

export function Field({
  label,
  hint,
  children,
  className = "",
}: {
  label: string;
  hint?: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={`field ${className}`}>
      <label>{label}</label>
      {children}
      {hint && <div className="field-hint">{hint}</div>}
    </div>
  );
}

export const inputClass = "input";

export function Card({
  children,
  className = "",
  onClick,
}: {
  children: ReactNode;
  className?: string;
  onClick?: () => void;
}) {
  return (
    <div className={`card ${className}`} onClick={onClick}>
      {children}
    </div>
  );
}

export function PageHeader({
  kicker,
  title,
  lead,
  action,
}: {
  kicker?: string;
  title: string;
  lead?: string;
  action?: ReactNode;
}) {
  return (
    <header className="flex items-end gap-5">
      <div className="min-w-0 flex-1">
        {kicker && <div className="kicker">{kicker}</div>}
        <h2 className="mt-1 text-[30px]">{title}</h2>
        {lead && <p className="mt-1.5 text-[13px] text-[var(--color-neutral-700)]">{lead}</p>}
      </div>
      {action}
    </header>
  );
}

export function ErrorBanner({ message }: { message: string | null }) {
  if (!message) return null;
  return <div className="notice notice-accent-2 mb-4">{message}</div>;
}

export function Modal({
  title,
  subtitle,
  onClose,
  children,
  width,
}: {
  title: string;
  subtitle?: string;
  onClose: () => void;
  children: ReactNode;
  width?: number;
}) {
  return (
    <div className="dialog-backdrop" onClick={onClose}>
      <div
        className="dialog"
        style={width ? { width: `min(${width}px, 100%)` } : undefined}
        onClick={(e) => e.stopPropagation()}
      >
        <div>
          <div className="dialog-title">{title}</div>
          {subtitle && (
            <div className="text-[13px] text-[var(--color-neutral-700)]">{subtitle}</div>
          )}
        </div>
        {children}
      </div>
    </div>
  );
}

export function Empty({ children }: { children: ReactNode }) {
  return (
    <div className="border border-dashed border-[var(--color-neutral-400)] py-12 text-center text-[13px] text-[var(--color-neutral-600)]">
      {children}
    </div>
  );
}

export function Stat({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div>
      <div className="stat-label">{label}</div>
      <div className="stat-value">{value}</div>
    </div>
  );
}

export const yen = (n: number) => `¥${n.toLocaleString("ja-JP")}`;
