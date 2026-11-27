"use client";

import { useState } from "react";

/** A labeled line of text, such as a URL or a command, with a copy button. */
export function CopyBlock({ label, text }: { label: string; text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="sample">
      <div className="field-head">
        <span className="field-label">{label}</span>
        <button
          type="button"
          className="btn btn-ghost"
          aria-label={`${label}をコピー`}
          onClick={() => {
            // Missing outside secure contexts (plain http on a LAN address).
            navigator.clipboard?.writeText(text).then(
              () => setCopied(true),
              () => setCopied(false),
            );
          }}
        >
          {copied ? "コピーしました" : "コピー"}
        </button>
      </div>
      <pre className="source-view sample-text">{text}</pre>
    </div>
  );
}
