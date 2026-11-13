"use client";

import { useIsOwner, useUserLibrary } from "./OwnerOnly";
import { RichText } from "./RichText";
import { SourceSections } from "./SourceSections";
import { useLatestVersion } from "./useLatestVersion";
import type { Library, LibraryVersion } from "@/lib/types";
import { Breadcrumbs, Card, EditLink, LoadState, VisibilityBadge } from "./ui";
import {
  libraryCodeEditPath,
  libraryEditPath,
  categoryPath,
  submissionUrlEditPath,
  userPath,
} from "@/lib/routes";

export interface LibraryViewData {
  library: Library;
  /** The latest code, or null when the library has none. */
  latest: LibraryVersion | null;
}

/**
 * The library page body. `initial` is what the server already fetched with the
 * visitor's cookies; without it (dev header auth) everything loads here.
 */
export function LibraryView({
  handle,
  libraryId,
  initial,
}: {
  handle: string;
  libraryId: string;
  initial?: LibraryViewData;
}) {
  const library = useUserLibrary(handle, libraryId, initial && { data: initial.library });
  const owner = useIsOwner(library.data);
  const latest = useLatestVersion(libraryId, initial && { data: initial.latest });

  return (
    <div className="page">
      {library.data ? (
        <>
          <Breadcrumbs
            items={[
              { label: `@${handle}`, href: userPath(handle) },
              { label: library.data.category.name, href: categoryPath(handle, library.data.category.id) },
              { label: library.data.name },
            ]}
          />
          <div className="page-title">
            <h1>{library.data.name}</h1>
            <VisibilityBadge visibility={library.data.visibility} />
          </div>
          <Card
            title="Description"
            action={owner ? <EditLink href={libraryEditPath(handle, libraryId)} what="Description" /> : null}
          >
            {library.data.description ? (
              <RichText source={library.data.description} />
            ) : (
              <p className="empty">
                {owner ? "説明はまだありません。「編集」から書けます。" : "説明はまだありません。"}
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
            editCodeHref={owner ? libraryCodeEditPath(handle, libraryId) : undefined}
            editUrlHref={owner ? submissionUrlEditPath(handle, libraryId) : undefined}
          />
        ) : latest.empty ? (
          <Card
            title="Code"
            action={owner ? <EditLink href={libraryCodeEditPath(handle, libraryId)} what="Code" /> : null}
          >
            <p className="empty">
              {owner ? "まだコードがありません。「編集」から登録してください。" : "まだコードがありません。"}
            </p>
          </Card>
        ) : (
          <LoadState loading={latest.loading} error={latest.error} notFound="コードが見つかりません。" />
        )
      ) : null}
    </div>
  );
}
