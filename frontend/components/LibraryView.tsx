"use client";

import Link from "next/link";
import { useNamespaceOwner } from "./OwnerOnly";
import { RichText } from "./RichText";
import { SourceSections } from "./SourceSections";
import { useLatestVersion } from "./useLatestVersion";
import { useResource } from "./useResource";
import type { Library, LibraryVersion, Namespace } from "@/lib/types";
import { Breadcrumbs, Card, LoadState, VisibilityBadge } from "./ui";
import {
  HOME_PATH,
  libraryCodeEditPath,
  libraryEditPath,
  namespacePath,
  submissionUrlEditPath,
} from "@/lib/routes";

export interface LibraryViewData {
  namespace: Namespace;
  library: Library;
  /** The latest code, or null when the library has none. */
  latest: LibraryVersion | null;
}

/**
 * The library page body. `initial` is what the server already fetched with the
 * visitor's cookies; without it (dev header auth) everything loads here.
 */
export function LibraryView({
  namespaceSlug,
  librarySlug,
  initial,
}: {
  namespaceSlug: string;
  librarySlug: string;
  initial?: LibraryViewData;
}) {
  const { owner } = useNamespaceOwner(namespaceSlug, initial && { data: initial.namespace });
  const library = useResource(
    `lib:${namespaceSlug}/${librarySlug}`,
    (api) => api.getLibrary(namespaceSlug, librarySlug),
    initial && { data: initial.library },
  );
  const latest = useLatestVersion(namespaceSlug, librarySlug, initial && { data: initial.latest });

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
