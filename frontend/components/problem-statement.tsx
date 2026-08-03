import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

type ProblemStatementProps = {
  children: string;
};

export function ProblemStatement({ children }: ProblemStatementProps) {
  return (
    <article className="problem-statement panel p-5">
      <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
        {children}
      </ReactMarkdown>
    </article>
  );
}
