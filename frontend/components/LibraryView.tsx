"use client";

import { useNamespaceOwner } from "./OwnerOnly";
import { RichText } from "./RichText";
import { SourceSections } from "./SourceSections";
import { useLatestVersion } from "./useLatestVersion";
import { useResource } from "./useResource";
import type { Library, LibraryVersion, Namespace } from "@/lib/types";
import { Breadcrumbs, Card, EditLink, LoadState, VisibilityBadge } from "./ui";
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
          </div>
          <Card
            title="Description"
            action={owner ? <EditLink href={libraryEditPath(namespaceSlug, librarySlug)} what="Description" /> : null}
          >
            {library.data.description ? (
              <RichText source={library.data.description} />
            ) : (
              <p className="empty">
                {owner ? "説明はまだありません。Edit から書けます。" : "説明はまだありません。"}
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
            editCodeHref={owner ? libraryCodeEditPath(namespaceSlug, librarySlug) : undefined}
            editUrlHref={owner ? submissionUrlEditPath(namespaceSlug, librarySlug) : undefined}
          />
        ) : latest.empty ? (
          <Card
            title="Code"
            action={owner ? <EditLink href={libraryCodeEditPath(namespaceSlug, librarySlug)} what="Code" /> : null}
          >
            <p className="empty">
              {owner ? "まだコードがありません。Edit から登録してください。" : "まだコードがありません。"}
            </p>
          </Card>
        ) : (
          <LoadState loading={latest.loading} error={latest.error} notFound="コードが見つかりません。" />
        )
      ) : null}
    </div>
  );
}
