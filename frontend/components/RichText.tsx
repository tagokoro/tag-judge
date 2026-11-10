"use client";

import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

/**
 * Renders a library description as Markdown with TeX math ($...$, $$...$$).
 * Raw HTML in the source is not rendered (react-markdown escapes it) and link
 * URLs go through react-markdown's default safe-URL filter.
 */
export function RichText({ source }: { source: string }) {
  return (
    <div className="rich-text">
      <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
        {source}
      </ReactMarkdown>
    </div>
  );
}
