// Display formatting. No React and no runtime imports from other local modules,
// so node:test can load this file directly.

const dateTime = new Intl.DateTimeFormat("ja-JP", {
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
});

/** "2026/11/10 20:02" in the viewer's time zone; the raw value if unparsable. */
export function formatDateTime(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.getTime()) ? iso : dateTime.format(date);
}

const STATUS_LABELS: Record<string, string> = {
  pending: "待機中",
  running: "判定中",
  ac: "AC",
  wa: "WA",
  tle: "TLE",
  mle: "MLE",
  re: "RE",
  ce: "CE",
  ie: "IE",
};

/** Judge status as shown in lists; unknown values pass through. */
export function statusLabel(status: string): string {
  return STATUS_LABELS[status] ?? status;
}

/** "accepted" | "rejected" | "waiting", used to color the status badge. */
export function statusTone(status: string): "accepted" | "rejected" | "waiting" {
  if (status === "ac") {
    return "accepted";
  }
  return status === "pending" || status === "running" ? "waiting" : "rejected";
}
