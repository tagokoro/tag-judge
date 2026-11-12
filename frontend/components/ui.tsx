"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { describeError, isApiRequestError } from "@/lib/api";
import type { Visibility } from "@/lib/types";

export interface Crumb {
  label: string;
  href?: string;
}

export function Breadcrumbs({ items }: { items: Crumb[] }) {
  return (
    <nav className="breadcrumbs" aria-label="パンくず">
      {items.map((item, index) => (
        <span key={`${index}-${item.label}`}>
          {index > 0 ? <span className="breadcrumbs-sep">/</span> : null}
          {item.href ? <Link href={item.href}>{item.label}</Link> : <span>{item.label}</span>}
        </span>
      ))}
    </nav>
  );
}

/** Loading / error state for a page-level resource. */
export function LoadState({
  loading,
  error,
  notFound,
}: {
  loading: boolean;
  error: unknown;
  notFound: string;
}) {
  if (loading) {
    return <p className="muted">読み込み中…</p>;
  }
  if (error === null) {
    return null;
  }
  if (isApiRequestError(error) && error.status === 404) {
    return <p className="empty">{notFound}</p>;
  }
  return (
    <p className="error" role="alert">
      {describeError(error)}
    </p>
  );
}

/** A page section. `owner` marks tools only the namespace owner can use. */
export function Card({
  title,
  note,
  owner = false,
  action,
  children,
}: {
  title: string;
  note?: ReactNode;
  owner?: boolean;
  /** A control shown at the right of the heading, such as an EditLink (「編集」). */
  action?: ReactNode;
  children: ReactNode;
}) {
  const heading = (
    <h2>
      {title}
      {owner ? <span className="owner-tag">所有者のみ</span> : null}
    </h2>
  );
  return (
    <section className={owner ? "section section-owner" : "section"}>
      {action ? (
        <div className="section-head">
          {heading}
          {action}
        </div>
      ) : (
        heading
      )}
      {note ? <p className="note">{note}</p> : null}
      {children}
    </section>
  );
}

/** The owner's 「編集」 button in a card heading; `what` names the card for screen readers. */
export function EditLink({ href, what }: { href: string; what: string }) {
  return (
    <Link className="btn btn-primary" href={href} aria-label={`${what} を編集`}>
      編集
    </Link>
  );
}

export function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="field">
      <span className="field-label">{label}</span>
      {children}
      {hint ? <span className="field-hint">{hint}</span> : null}
    </label>
  );
}

export function Button({
  children,
  onClick,
  disabled,
  variant = "default",
}: {
  children: ReactNode;
  onClick: () => void;
  disabled?: boolean;
  variant?: "default" | "primary" | "ghost" | "danger";
}) {
  return (
    <button
      type="button"
      className={`btn btn-${variant}`}
      onClick={onClick}
      disabled={disabled}
    >
      {children}
    </button>
  );
}

export function ErrorNote({ error }: { error: string | null }) {
  if (!error) {
    return null;
  }
  return (
    <p className="error" role="alert">
      {error}
    </p>
  );
}

export function VisibilityBadge({ visibility }: { visibility: Visibility }) {
  return (
    <span className={`badge badge-${visibility}`}>
      {visibility}
    </span>
  );
}

export function formatError(error: unknown): string {
  return describeError(error);
}
