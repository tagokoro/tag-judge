"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { use, useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { useAsyncAction } from "@/components/useAsyncAction";
import { useResource } from "@/components/useResource";
import {
  Breadcrumbs,
  Button,
  Card,
  ErrorNote,
  Field,
  LoadState,
  VisibilityBadge,
} from "@/components/ui";
import { HOME_PATH, libraryPath } from "@/lib/routes";
import { isOwner } from "@/lib/session";
import type { Visibility } from "@/lib/types";

export default function NamespacePage({ params }: { params: Promise<{ namespace: string }> }) {
  const { namespace: namespaceSlug } = use(params);
  const { actor } = useAuth();
  const namespace = useResource(`ns:${namespaceSlug}`, (api) => api.getNamespace(namespaceSlug));
  const libraries = useResource(`ns:${namespaceSlug}:libraries`, (api) =>
    api.listLibraries(namespaceSlug),
  );

  const owner = namespace.data !== null && isOwner(actor, namespace.data.owner_user_id);

  return (
    <div className="page">
      <Breadcrumbs items={[{ label: "ホーム", href: HOME_PATH }, { label: namespaceSlug }]} />
      {namespace.data ? (
        <div className="page-title">
          <h1>{namespace.data.display_name}</h1>
          <span className="page-sub">@{namespace.data.slug}</span>
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
                  ? "まだ Library がありません。下のフォームから最初の Library を作成してください。"
                  : "公開されている Library はまだありません。"}
              </p>
            ) : (
              <ul className="lib-list">
                {libraries.data.map((library) => (
                  <li key={library.id}>
                    <Link className="lib-row" href={libraryPath(namespaceSlug, library.slug)}>
                      <span>
                        <span className="lib-name">{library.name}</span>
                        <span className="lib-slug">{library.slug}</span>
                      </span>
                      <VisibilityBadge visibility={library.visibility} />
                      {library.description ? (
                        <span className="lib-desc">{library.description}</span>
                      ) : null}
                    </Link>
                  </li>
                ))}
              </ul>
            )
          ) : (
            <LoadState loading={libraries.loading} error={libraries.error} notFound="" />
          )}
        </Card>
      ) : null}

      {owner ? <CreateLibraryForm namespaceSlug={namespaceSlug} /> : null}
    </div>
  );
}

function CreateLibraryForm({ namespaceSlug }: { namespaceSlug: string }) {
  const { client } = useAuth();
  const router = useRouter();
  const { busy, error, run } = useAsyncAction();
  const [slug, setSlug] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [visibility, setVisibility] = useState<Visibility>("private");

  return (
    <Card title="Library を作成" owner>
      <div className="row">
        <Field label="slug" hint="Namespace 内で一意 / 2〜48 文字">
          <input
            value={slug}
            onChange={(event) => setSlug(event.target.value)}
            spellCheck={false}
            placeholder="union-find"
          />
        </Field>
        <Field label="名前" hint="1〜80 文字">
          <input
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="UnionFind"
          />
        </Field>
        <Field label="公開範囲">
          <select
            value={visibility}
            onChange={(event) => setVisibility(event.target.value as Visibility)}
          >
            <option value="private">非公開（自分だけ）</option>
            <option value="public">公開（誰でも見られる）</option>
          </select>
        </Field>
      </div>
      <Field label="説明" hint="2000 文字まで">
        <textarea value={description} onChange={(event) => setDescription(event.target.value)} />
      </Field>
      <div className="row">
        <Button
          variant="primary"
          onClick={() =>
            run(async (signal) => {
              const created = await client.withSignal(signal).createLibrary(namespaceSlug, {
                slug: slug.trim(),
                name,
                description,
                visibility,
              });
              router.push(libraryPath(namespaceSlug, created.slug));
            })
          }
          disabled={busy}
        >
          作成
        </Button>
      </div>
      <ErrorNote error={error} />
    </Card>
  );
}
