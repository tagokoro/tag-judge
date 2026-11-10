"use client";

import type { ReactNode } from "react";
import { describeError } from "@/lib/api";
import type { Visibility } from "@/lib/types";

export function Card({
  title,
  note,
  children,
}: {
  title: string;
  note?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="card">
      <h2>{title}</h2>
      {note ? <p className="note">{note}</p> : null}
      {children}
    </section>
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
  variant?: "default" | "primary" | "ghost";
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

/** Renders data as escaped TEXT only (JSON in a <pre>). Never uses innerHTML. */
export function ResultView({ data, label }: { data: unknown; label?: string }) {
  return (
    <div className="result-block">
      {label ? <span className="result-label">{label}</span> : null}
      <pre className="result">{JSON.stringify(data, null, 2)}</pre>
    </div>
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
      {visibility === "public" ? "public" : "private"}
    </span>
  );
}

export function formatError(error: unknown): string {
  return describeError(error);
}
