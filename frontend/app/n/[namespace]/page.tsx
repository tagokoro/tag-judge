"use client";

import Link from "next/link";
import { use } from "react";
import { LibraryList } from "@/components/lists";
import { useNamespaceOwner } from "@/components/OwnerOnly";
import { useResource } from "@/components/useResource";
import { Breadcrumbs, Card, LoadState } from "@/components/ui";
import { HOME_PATH, NEW_LIBRARY_PATH } from "@/lib/routes";

export default function NamespacePage({ params }: { params: Promise<{ namespace: string }> }) {
  const { namespace: namespaceSlug } = use(params);
  const { namespace, owner } = useNamespaceOwner(namespaceSlug);
  const libraries = useResource(`ns:${namespaceSlug}:libraries`, (api) =>
    api.listLibraries(namespaceSlug),
  );

  return (
    <div className="page">
      <Breadcrumbs items={[{ label: "ライブラリ一覧", href: HOME_PATH }, { label: namespaceSlug }]} />
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
                  : "公開されている Library はまだありません。"}
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
