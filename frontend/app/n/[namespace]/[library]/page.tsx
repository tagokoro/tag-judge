"use client";

import Link from "next/link";
import { use } from "react";
import { useNamespaceOwner } from "@/components/OwnerOnly";
import { RichText } from "@/components/RichText";
import { SourceSections } from "@/components/SourceSections";
import { useLatestVersion } from "@/components/useLatestVersion";
import { useResource } from "@/components/useResource";
import { Breadcrumbs, Card, LoadState, VisibilityBadge } from "@/components/ui";
import {
  HOME_PATH,
  libraryCodeEditPath,
  libraryEditPath,
  namespacePath,
  submissionUrlEditPath,
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
  const latest = useLatestVersion(namespaceSlug, librarySlug);

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
                <Link className="btn btn-primary" href={libraryCodeEditPath(namespaceSlug, librarySlug)}>
                  コードを更新
                </Link>
              </div>
            ) : null}
          </div>
          <Card title="Description">
            {library.data.description ? (
              <RichText source={library.data.description} />
            ) : (
              <p className="empty">
                {owner ? "説明はまだありません。「設定を編集」から書けます。" : "説明はまだありません。"}
              </p>
            )}
          </Card>
        </>
      ) : (
        <LoadState
          loading={library.loading}
          error={library.error}
          notFound="この Library は見つかりません（private の Library は所有者にしか見えません）。"
        />
      )}

      {library.data ? (
        latest.data ? (
          <SourceSections
            version={latest.data}
            editUrlHref={owner ? submissionUrlEditPath(namespaceSlug, librarySlug) : undefined}
          />
        ) : latest.empty ? (
          <Card title="Code">
            <p className="empty">
              {owner ? "まだコードがありません。「コードを更新」から登録してください。" : "まだコードがありません。"}
            </p>
          </Card>
        ) : (
          <LoadState loading={latest.loading} error={latest.error} notFound="コードが見つかりません。" />
        )
      ) : null}
    </div>
  );
}
