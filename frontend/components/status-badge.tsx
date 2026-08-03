import type { SubmissionStatus, Visibility } from "@/lib/types";

const statusClass: Record<SubmissionStatus, string> = {
  pending: "border-ink/15 bg-ink/5 text-ink/70",
  running: "border-warn/20 bg-warn/10 text-warn",
  ac: "border-accent/20 bg-accent/10 text-accent",
  wa: "border-danger/20 bg-danger/10 text-danger",
  tle: "border-danger/20 bg-danger/10 text-danger",
  mle: "border-danger/20 bg-danger/10 text-danger",
  re: "border-danger/20 bg-danger/10 text-danger",
  ce: "border-danger/20 bg-danger/10 text-danger",
  ie: "border-danger/20 bg-danger/10 text-danger"
};

const visibilityClass: Record<Visibility, string> = {
  public: "border-accent/20 bg-accent/10 text-accent",
  private: "border-ink/15 bg-ink/5 text-ink/70",
  group: "border-warn/20 bg-warn/10 text-warn"
};

export function StatusBadge({ value }: { value: SubmissionStatus }) {
  return <span className={`inline-flex rounded px-2 py-1 text-xs font-semibold ${statusClass[value]}`}>{value}</span>;
}

export function VisibilityBadge({ value }: { value: Visibility }) {
  return <span className={`inline-flex rounded px-2 py-1 text-xs font-semibold ${visibilityClass[value]}`}>{value}</span>;
}
