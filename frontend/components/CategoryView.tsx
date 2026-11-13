"use client";

import Link from "next/link";
import { LibraryList } from "./lists";
import { useNamespaceOwner } from "./OwnerOnly";
import { useResource } from "./useResource";
import { Breadcrumbs, Card, LoadState } from "./ui";
import { HOME_PATH, NEW_LIBRARY_PATH } from "@/lib/routes";
import type { Library, Namespace } from "@/lib/types";

/**
 * The namespace page body. `initial` is what the server already fetched with
 * the visitor's cookies; the list is fetched here again only for a dev-header
 * actor, who may own private libraries the server cannot see.
 */
export function NamespaceView({
  namespaceSlug,
  initial,
}: {
  namespaceSlug: string;
  initial: { namespace: Namespace; libraries: Library[] };
}) {
  const { namespace, owner } = useNamespaceOwner(namespaceSlug, { data: initial.namespace });
  const libraries = useResource(
    `ns:${namespaceSlug}:libraries`,
    (api) => api.listLibraries(namespaceSlug),
    { data: initial.libraries },
  );

  return (
    <div className="page">
      <Breadcrumbs items={[{ label: "Libraries", href: HOME_PATH }, { label: namespaceSlug }]} />
      {namespace.data ? (
        <div className="page-title">
          <h1>{namespace.data.display_name}</h1>
          <span className="page-sub">@{namespace.data.slug}</span>
          {owner ? (
            <div className="page-actions">
              <Link className="btn btn-primary" href={NEW_LIBRARY_PATH}>
                Library を作成
              </Link>
            </div>
          ) : null}
        </div>
      ) : (
        <LoadState
          loading={namespace.loading}
          error={namespace.error}
          notFound="この Namespace は見つかりません。"
        />
      )}

      {namespace.data ? (
        <Card title="Library">
          {libraries.data ? (
            libraries.data.length === 0 ? (
              <p className="empty">
                {owner
                  ? "まだ Library がありません。「Library を作成」から最初の Library を作ってください。"
                  : "public な Library はまだありません。"}
              </p>
            ) : (
              <LibraryList libraries={libraries.data} />
            )
          ) : (
            <LoadState loading={libraries.loading} error={libraries.error} notFound="" />
          )}
        </Card>
      ) : null}
    </div>
  );
}
