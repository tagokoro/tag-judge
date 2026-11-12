// Display formatting. No React and no runtime imports from other local modules,
// so node:test can load this file directly.

// A fixed zone: dates are rendered on the server too, and the server's zone
// must not leak into the page (or differ from the browser's at hydration).
const dateTime = new Intl.DateTimeFormat("ja-JP", {
  timeZone: "Asia/Tokyo",
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
});

/** "2026/11/10 20:02" in Japan time; the raw value if unparsable. */
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

/**
 * A one-paragraph plain-text summary of a Markdown + TeX description, for list
 * rows: the first paragraph of prose, with inline Markdown and math delimiters
 * dropped. Headings, list items, code blocks and display math are skipped.
 */
export function descriptionSummary(markdown: string): string {
  const paragraphs = markdown
    .replace(/```[\s\S]*?```/g, "\n\n")
    .replace(/\$\$[\s\S]*?\$\$/g, "\n\n")
    .split(/\n\s*\n/);
  for (const paragraph of paragraphs) {
    const lines = paragraph
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line !== "" && !/^(#{1,6}\s|[-*+]\s|\d+\.\s|>|\|)/.test(line));
    if (lines.length === 0) {
      continue;
    }
    return lines
      .join(" ")
      .replace(/!?\[([^\]]*)\]\([^)]*\)/g, "$1")
      .replace(/(\*\*|__|\*|_|~~|`)/g, "")
      .replace(/\$([^$]*)\$/g, "$1")
      .replace(/\s+/g, " ")
      .trim();
  }
  return "";
}
