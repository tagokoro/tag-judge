"use client";

import Link from "next/link";
import { descriptionSummary, formatDateTime, statusLabel, statusTone } from "@/lib/format";
import { libraryPath, namespacePath } from "@/lib/routes";
import type { Library, Problem } from "@/lib/types";
import { VisibilityBadge } from "./ui";

/** Libraries as rows. `showNamespace` adds the owner for site-wide lists. */
export function LibraryList({
  libraries,
  showNamespace = false,
}: {
  libraries: Library[];
  showNamespace?: boolean;
}) {
  return (
    <ul className="lib-list">
      {libraries.map((library) => {
        // Rows are links, so the description is a plain-text summary, never rendered Markdown.
        const summary = descriptionSummary(library.description);
        return (
          <li key={library.id}>
            <Link className="lib-row" href={libraryPath(library.namespace_slug, library.slug)}>
              <span>
                <span className="lib-name">{library.name}</span>
                <span className="lib-slug">
                  {showNamespace ? `${library.namespace_slug}/` : ""}
                  {library.slug}
                </span>
              </span>
              <VisibilityBadge visibility={library.visibility} />
              {summary ? <span className="lib-desc">{summary}</span> : null}
            </Link>
          </li>
        );
      })}
    </ul>
  );
}

export function ProblemList({ problems, showVisibility = false }: { problems: Problem[]; showVisibility?: boolean }) {
  return (
    <table className="table">
      <thead>
        <tr>
          <th scope="col" className="col-shrink">ID</th>
          <th scope="col">問題</th>
          <th scope="col" className="col-shrink">制限</th>
          {showVisibility ? <th scope="col">Visibility</th> : null}
        </tr>
      </thead>
      <tbody>
        {problems.map((problem) => (
          <tr key={problem.id}>
            <td className="num col-shrink" data-label="ID">{problem.id}</td>
            <td className="cell-main" data-label="問題">{problem.title}</td>
            <td className="num muted col-shrink" data-label="制限">
              {problem.time_limit_ms / 1000} 秒 / {Math.round(problem.memory_limit_kb / 1024)} MB
            </td>
            {showVisibility ? (
              <td data-label="Visibility">
                <VisibilityBadge visibility={problem.visibility === "public" ? "public" : "private"} />
              </td>
            ) : null}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export interface SubmissionRow {
  id: number;
  problem: string;
  submitter?: string | null;
  language: string;
  status: string;
  score: number;
  created_at: string;
}

export function SubmissionTable({
  submissions,
  showSubmitter = false,
}: {
  submissions: SubmissionRow[];
  showSubmitter?: boolean;
}) {
  return (
    <table className="table">
      <thead>
        <tr>
          <th scope="col" className="col-shrink">提出日時</th>
          <th scope="col">問題</th>
          {showSubmitter ? <th scope="col">提出者</th> : null}
          <th scope="col">言語</th>
          <th scope="col">結果</th>
          <th scope="col">得点</th>
        </tr>
      </thead>
      <tbody>
        {submissions.map((submission) => (
          <tr key={submission.id}>
            <td className="num col-shrink" data-label="提出日時">{formatDateTime(submission.created_at)}</td>
            <td className="cell-main" data-label="問題">{submission.problem}</td>
            {showSubmitter ? (
              <td data-label="提出者">
                {submission.submitter ? (
                  <Link href={namespacePath(submission.submitter)}>{submission.submitter}</Link>
                ) : (
                  <span className="muted">—</span>
                )}
              </td>
            ) : null}
            <td data-label="言語">{submission.language}</td>
            <td data-label="結果">
              <span className={`status status-${statusTone(submission.status)}`}>
                {statusLabel(submission.status)}
              </span>
            </td>
            <td className="num" data-label="得点">{submission.score}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
