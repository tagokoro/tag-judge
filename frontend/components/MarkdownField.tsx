"use client";

import { useState } from "react";
import { RichText } from "./RichText";

/** A Markdown + TeX textarea with a preview tab. */
export function MarkdownField({
  label,
  hint,
  value,
  onChange,
}: {
  label: string;
  hint?: string;
  value: string;
  onChange: (value: string) => void;
}) {
  const [preview, setPreview] = useState(false);

  return (
    <div className="field">
      <div className="field-head">
        <span className="field-label">{label}</span>
        <div className="tabs" role="tablist" aria-label={`${label}の表示`}>
          <button type="button" role="tab" aria-selected={!preview} onClick={() => setPreview(false)}>
            書く
          </button>
          <button type="button" role="tab" aria-selected={preview} onClick={() => setPreview(true)}>
            プレビュー
          </button>
        </div>
      </div>
      {preview ? (
        <div className="preview">
          {value.trim() ? <RichText source={value} /> : <p className="muted">プレビューする内容がありません。</p>}
        </div>
      ) : (
        <textarea value={value} onChange={(event) => onChange(event.target.value)} aria-label={label} />
      )}
      {hint ? <span className="field-hint">{hint}</span> : null}
    </div>
  );
}
