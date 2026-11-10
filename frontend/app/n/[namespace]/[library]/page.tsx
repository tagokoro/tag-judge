"use client";

import Link from "next/link";
import { use } from "react";
import { useNamespaceOwner } from "@/components/OwnerOnly";
import { RichText } from "@/components/RichText";
import { useResource } from "@/components/useResource";
import { Breadcrumbs, Card, LoadState, VisibilityBadge } from "@/components/ui";
import { formatDateTime } from "@/lib/format";
import {
  HOME_PATH,
  libraryEditPath,
  namespacePath,
  newVersionPath,
  versionPath,
} from "@/lib/routes";

export default function LibraryPage({
  params,
}: {
  params: Promise<{ namespace: string; library: string }>;
}) {
  const { namespace: namespaceSlug, library: librarySlug } = use(params);
  const { owner } = useNamespaceOwner(namespaceSlug);
  const library = useResource(`lib:${namespaceSlug}/${librarySlug}`, (api) =>
    api.getLibrary(namespaceSlug, librarySlug),
  );
  const versions = useResource(`lib:${namespaceSlug}/${librarySlug}:versions`, (api) =>
    api.listVersions(namespaceSlug, librarySlug),
  );

  return (
    <div className="page">
      <Breadcrumbs
        items={[
          { label: "Libraries", href: HOME_PATH },
          { label: namespaceSlug, href: namespacePath(namespaceSlug) },
          { label: librarySlug },
        ]}
      />
      {library.data ? (
        <>
          <div className="page-title">
            <h1>{library.data.name}</h1>
            <VisibilityBadge visibility={library.data.visibility} />
            {owner ? (
              <div className="page-actions">
                <Link className="btn" href={libraryEditPath(namespaceSlug, librarySlug)}>
                  設定を編集
                </Link>
                <Link className="btn btn-primary" href={newVersionPath(namespaceSlug, librarySlug)}>
                  Version を追加
                </Link>
              </div>
            ) : null}
          </div>
          {library.data.description ? (
            <Card title="説明">
              <RichText source={library.data.description} />
            </Card>
          ) : null}
        </>
      ) : (
        <LoadState
          loading={library.loading}
          error={library.error}
          notFound="この Library は見つかりません（private の Library は所有者にしか見えません）。"
        />
      )}

      {library.data ? (
        <Card title="Version">
          {versions.data ? (
            versions.data.length === 0 ? (
              <p className="empty">
                {owner
                  ? "まだ Version がありません。「Version を追加」からソースを登録すると v1 になります。"
                  : "まだ Version がありません。"}
              </p>
            ) : (
              <ol className="version-line">
                {versions.data.map((version) => (
                  <li key={version.id}>
                    <Link
                      className="version-node"
                      href={versionPath(namespaceSlug, librarySlug, version.version_number)}
                    >
                      <span className="version-dot">v{version.version_number}</span>
                      <span className="version-meta">
                        <span className="num">{formatDateTime(version.created_at)}</span>
                        <span className="muted num">{version.byte_size} バイト</span>
                      </span>
                    </Link>
                  </li>
                ))}
              </ol>
            )
          ) : (
            <LoadState loading={versions.loading} error={versions.error} notFound="" />
          )}
        </Card>
      ) : null}
    </div>
  );
}
