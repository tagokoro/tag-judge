"use client";

import Link from "next/link";
import { use } from "react";
import { CodeView } from "@/components/CodeEditor";
import { useNamespaceOwner } from "@/components/OwnerOnly";
import { useResource } from "@/components/useResource";
import { Breadcrumbs, Card, LoadState } from "@/components/ui";
import { formatDateTime } from "@/lib/format";
import {
  HOME_PATH,
  libraryPath,
  namespacePath,
  parseVersionParam,
  versionEditPath,
} from "@/lib/routes";

export default function VersionPage({
  params,
}: {
  params: Promise<{ namespace: string; library: string; version: string }>;
}) {
  const { namespace: namespaceSlug, library: librarySlug, version: rawVersion } = use(params);
  const versionNumber = parseVersionParam(rawVersion);
  const { owner } = useNamespaceOwner(namespaceSlug);
  const version = useResource(
    versionNumber === null ? null : `ver:${namespaceSlug}/${librarySlug}/${versionNumber}`,
    (api) => api.getVersion(namespaceSlug, librarySlug, versionNumber ?? 0),
  );

  return (
    <div className="page">
      <Breadcrumbs
        items={[
          { label: "Libraries", href: HOME_PATH },
          { label: namespaceSlug, href: namespacePath(namespaceSlug) },
          { label: librarySlug, href: libraryPath(namespaceSlug, librarySlug) },
          { label: `v${rawVersion}` },
        ]}
      />
      {versionNumber === null ? (
        <p className="empty">Version 番号が正しくありません。v1 のような番号で開いてください。</p>
      ) : version.data ? (
        <>
          <div className="page-title">
            <h1>
              {librarySlug} v{version.data.version_number}
            </h1>
            {owner ? (
              <div className="page-actions">
                <Link
                  className="btn"
                  href={versionEditPath(namespaceSlug, librarySlug, version.data.version_number)}
                >
                  提出 URL を編集
                </Link>
              </div>
            ) : null}
          </div>
          <Card title="ソース">
            <dl className="kv">
              <dt>登録日時</dt>
              <dd className="num">{formatDateTime(version.data.created_at)}</dd>
              <dt>サイズ</dt>
              <dd className="num">{version.data.byte_size} バイト</dd>
              <dt>SHA-256</dt>
              <dd className="hash">{version.data.content_sha256}</dd>
            </dl>
            {/* Source is untrusted text: CodeMirror shows it as text, never as HTML. */}
            <div className="code-block">
              <CodeView value={version.data.source_code} label="ソース" />
            </div>
          </Card>
          <Card
            title="関連する提出 URL"
            note="この版を使った提出の URL です。所有者が登録したもので、提出の内容や判定結果は確認していません。"
          >
            {version.data.submission_url ? (
              <dl className="kv">
                <dt>URL</dt>
                {/* Self-declared, untrusted text: shown as text, never fetched or followed automatically. */}
                <dd className="url-text">{version.data.submission_url}</dd>
                <dt>更新日時</dt>
                <dd className="num">
                  {version.data.submission_url_updated_at
                    ? formatDateTime(version.data.submission_url_updated_at)
                    : ""}
                </dd>
              </dl>
            ) : (
              <p className="empty">提出 URL は登録されていません。</p>
            )}
          </Card>
        </>
      ) : (
        <LoadState
          loading={version.loading}
          error={version.error}
          notFound="この Version は見つかりません（private の Library は所有者にしか見えません）。"
        />
      )}
    </div>
  );
}
