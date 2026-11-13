"use client";

import Link from "next/link";
import { LibraryList } from "./lists";
import { useIsOwner, useUserCategory } from "./OwnerOnly";
import { RichText } from "./RichText";
import { useResource } from "./useResource";
import { Breadcrumbs, Card, EditLink, LoadState } from "./ui";
import { categoryEditPath, newLibraryPath, userPath } from "@/lib/routes";
import type { Library, Category } from "@/lib/types";

/**
 * The category page body. `initial` is what the server already fetched with
 * the visitor's cookies; without it (a category only a dev-header owner can
 * see) everything loads here.
 */
export function CategoryView({
  handle,
  categoryId,
  initial,
}: {
  handle: string;
  categoryId: string;
  initial?: { category: Category; libraries: Library[] };
}) {
  const category = useUserCategory(handle, categoryId, initial && { data: initial.category });
  const owner = useIsOwner(category.data);
  const libraries = useResource(
    category.data ? `category:${categoryId}:libraries` : null,
    (api) => api.listLibraries(categoryId),
    initial && { data: initial.libraries },
  );

  return (
    <div className="page">
      <Breadcrumbs
        items={[{ label: `@${handle}`, href: userPath(handle) }, { label: category.data?.name ?? "カテゴリ" }]}
      />
      {category.data ? (
        <>
          <div className="page-title">
            <h1>{category.data.name}</h1>
            <span className="page-sub">@{handle}</span>
          </div>
          {category.data.description || owner ? (
            <Card
              title="Description"
              action={owner ? <EditLink href={categoryEditPath(handle, categoryId)} what="カテゴリ" /> : null}
            >
              {category.data.description ? (
                <RichText source={category.data.description} />
              ) : (
                <p className="empty">説明はまだありません。「編集」から書けます。</p>
              )}
            </Card>
          ) : null}
          <Card
            title="Libraries"
            action={
              owner ? (
                <Link className="btn btn-primary" href={newLibraryPath(categoryId)}>
                  Library を作成
                </Link>
              ) : null
            }
          >
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
        </>
      ) : (
        <LoadState loading={category.loading} error={category.error} notFound="このカテゴリは見つかりません。" />
      )}
    </div>
  );
}
