"use client";

import Link from "next/link";
import { useAuth } from "./AuthProvider";
import { useResource } from "./useResource";
import { Avatar, Card, EditLink, LoadState } from "./ui";
import { descriptionSummary } from "@/lib/format";
import { categoryPath, newLibraryPath, SETTINGS_PROFILE_PATH } from "@/lib/routes";
import { isOwner } from "@/lib/session";
import type { Category, UserProfile } from "@/lib/types";

/**
 * The user page body: the profile and the categories the visitor may see.
 * `initial` is what the server fetched with the visitor's cookies; a
 * dev-header owner, whom the server cannot see, loads the categories again.
 */
export function UserView({ initial }: { initial: { profile: UserProfile; categories: Category[] } }) {
  const { actor } = useAuth();
  const { profile } = initial;
  const owner = isOwner(actor, profile.handle);
  const categories = useResource(
    `user:${profile.handle}:categories`,
    (api) => api.listCategories(profile.handle),
    { data: initial.categories },
  );

  return (
    <div className="page">
      <section className="section profile-head">
        <Avatar name={profile.handle} large />
        <div className="profile-name">
          <h1>@{profile.handle}</h1>
        </div>
        {owner ? (
          <div className="page-actions">
            <EditLink href={SETTINGS_PROFILE_PATH} what="プロフィール" />
          </div>
        ) : null}
      </section>

      <Card
        title="Categories"
        action={
          owner ? (
            <Link className="btn btn-primary" href={newLibraryPath()}>
              Library を作成
            </Link>
          ) : null
        }
      >
        {categories.data ? (
          categories.data.length === 0 ? (
            <p className="empty">
              {owner
                ? "まだ Library がありません。「Library を作成」から作ると、カテゴリもそこで作られます。"
                : "public な Library はまだありません。"}
            </p>
          ) : (
            <CategoryList categories={categories.data} />
          )
        ) : (
          <LoadState loading={categories.loading} error={categories.error} notFound="" />
        )}
      </Card>
    </div>
  );
}

/** Categories as rows linking to their pages. */
function CategoryList({ categories }: { categories: Category[] }) {
  return (
    <ul className="lib-list">
      {categories.map((category) => {
        // Rows are links, so the description is a plain-text summary, never rendered Markdown.
        const summary = descriptionSummary(category.description);
        return (
          <li key={category.id}>
            <Link className="lib-row" href={categoryPath(category.owner, category.id)}>
              <span className="lib-name">{category.name}</span>
              {summary ? <span className="lib-desc">{summary}</span> : null}
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
