"use client";

import Link from "next/link";
import { formatDateTime } from "@/lib/format";
import { isValidSubmissionUrl } from "@/lib/submissionUrl";
import type { LibraryVersion } from "@/lib/types";
import { CodeView } from "./CodeEditor";
import { Card } from "./ui";

/** The code and submission-URL cards of a library's latest code. */
export function SourceSections({
  version,
  editUrlHref,
}: {
  version: LibraryVersion;
  /** Shown as an edit button in the submission-URL card when set (owner only). */
  editUrlHref?: string;
}) {
  return (
    <>
      <Card title="Code">
        <dl className="kv">
          <dt>更新日時</dt>
          <dd className="num">{formatDateTime(version.created_at)}</dd>
          <dt>サイズ</dt>
          <dd className="num">{version.byte_size} バイト</dd>
          <dt>SHA-256</dt>
          <dd className="hash">{version.content_sha256}</dd>
        </dl>
        {/* Source is untrusted text: CodeMirror shows it as text, never as HTML. */}
        <div className="code-block">
          <CodeView value={version.source_code} label="コード" />
        </div>
      </Card>
      <Card title="Verification">
        {version.submission_url ? (
          <dl className="kv">
            <dt>URL</dt>
            <dd className="url-text">
              {/* Self-declared, untrusted URL: linked only when it is plain
                  http(s), opened in a new tab with no referrer or opener. */}
              {isValidSubmissionUrl(version.submission_url) ? (
                <a href={version.submission_url} target="_blank" rel="noopener noreferrer nofollow">
                  {version.submission_url}
                </a>
              ) : (
                version.submission_url
              )}
            </dd>
            <dt>更新日時</dt>
            <dd className="num">
              {version.submission_url_updated_at ? formatDateTime(version.submission_url_updated_at) : ""}
            </dd>
          </dl>
        ) : (
          <p className="empty">提出 URL は登録されていません。</p>
        )}
        {editUrlHref ? (
          <div className="card-actions">
            <Link className="btn" href={editUrlHref}>
              提出 URL を編集
            </Link>
          </div>
        ) : null}
      </Card>
    </>
  );
}
