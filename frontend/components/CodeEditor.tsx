"use client";

import { python } from "@codemirror/lang-python";
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { tags } from "@lezer/highlight";
import CodeMirror, { EditorView } from "@uiw/react-codemirror";
import { useSyncExternalStore } from "react";

// Syntax colors tuned to the white + 水色 palette (WCAG AA on white).
const highlight = HighlightStyle.define([
  { tag: tags.keyword, color: "#0f5e8c", fontWeight: "600" },
  { tag: [tags.definition(tags.variableName), tags.function(tags.definition(tags.variableName))], color: "#0b6b6b" },
  { tag: [tags.string, tags.special(tags.string)], color: "#9a4a00" },
  { tag: [tags.number, tags.bool, tags.null], color: "#7a3fa0" },
  { tag: tags.comment, color: "#5f7385", fontStyle: "italic" },
  { tag: [tags.operator, tags.punctuation], color: "#3f566a" },
  { tag: tags.className, color: "#0b6b6b", fontWeight: "600" },
]);

const frame = EditorView.theme({
  "&": {
    border: "1px solid var(--line)",
    borderRadius: "var(--radius-control)",
    backgroundColor: "var(--paper)",
    fontSize: "0.88rem",
  },
  "&.cm-focused": {
    outline: "none",
    borderColor: "var(--deep)",
    boxShadow: "0 0 0 3px var(--sky)",
  },
  ".cm-scroller": { fontFamily: "var(--font-code)", lineHeight: "1.6" },
  ".cm-gutters": {
    backgroundColor: "var(--mist)",
    color: "var(--muted)",
    borderRight: "1px solid var(--line)",
    borderRadius: "var(--radius-control) 0 0 var(--radius-control)",
  },
  ".cm-activeLine, .cm-activeLineGutter": { backgroundColor: "rgb(220 240 250 / 0.45)" },
});

const extensions = [python(), syntaxHighlighting(highlight), frame];

/** Python source editor with syntax highlighting. */
export function CodeEditor({
  value,
  onChange,
  label,
}: {
  value: string;
  onChange: (value: string) => void;
  label: string;
}) {
  return (
    <CodeMirror
      value={value}
      onChange={onChange}
      extensions={extensions}
      minHeight="16rem"
      maxHeight="40rem"
      aria-label={label}
      basicSetup={{ foldGutter: false, autocompletion: false }}
    />
  );
}

const noSubscribe = () => () => {};

/** False in the server render and during hydration, true once running in the browser. */
function useInBrowser(): boolean {
  return useSyncExternalStore(noSubscribe, () => true, () => false);
}

/**
 * Read-only, highlighted view of a stored source. Shown as text, never as HTML.
 * CodeMirror only draws in the browser, so the server sends a plain <pre> with
 * the same text, which CodeMirror replaces after hydration.
 */
export function CodeView({ value, label }: { value: string; label: string }) {
  const inBrowser = useInBrowser();
  if (!inBrowser) {
    return (
      <pre className="source-view" aria-label={label}>
        {value}
      </pre>
    );
  }
  return (
    <CodeMirror
      value={value}
      extensions={extensions}
      editable={false}
      readOnly
      aria-label={label}
      basicSetup={{ foldGutter: false, highlightActiveLine: false, highlightActiveLineGutter: false }}
    />
  );
}
