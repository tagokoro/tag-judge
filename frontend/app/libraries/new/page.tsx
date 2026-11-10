"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { LoginRequired } from "@/components/LoginRequired";
import { MarkdownField } from "@/components/MarkdownField";
import { useAsyncAction } from "@/components/useAsyncAction";
import { useResource } from "@/components/useResource";
import { Button, Card, ErrorNote, Field, LoadState } from "@/components/ui";
import { isApiRequestError } from "@/lib/api";
import { libraryPath, ME_PATH, namespacePath } from "@/lib/routes";
import type { Visibility } from "@/lib/types";

/** Creates a library in the signed-in user's own namespace. */
export default function NewLibraryPage() {
  const { actor } = useAuth();
  const mine = useResource(actor ? "me/namespace" : null, (api) => api.getMyNamespace());

  if (!actor) {
    return <LoginRequired title="Library を作成" />;
  }
  if (isApiRequestError(mine.error) && mine.error.status === 404) {
    return (
      <div className="page">
        <h1>Library を作成</h1>
        <p className="empty">
          Library を置く Namespace がまだありません。<Link href={ME_PATH}>先に Namespace を作成してください。</Link>
        </p>
      </div>
    );
  }
  return (
    <div className="page">
      <h1>Library を作成</h1>
      {mine.data ? (
        <NewLibraryForm namespaceSlug={mine.data.slug} />
      ) : (
        <LoadState loading={mine.loading} error={mine.error} notFound="" />
      )}
    </div>
  );
}

function NewLibraryForm({ namespaceSlug }: { namespaceSlug: string }) {
  const { client } = useAuth();
  const router = useRouter();
  const { busy, error, run } = useAsyncAction();
  const [slug, setSlug] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [visibility, setVisibility] = useState<Visibility>("private");

  return (
    <Card title={`@${namespaceSlug} に作成`} owner>
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
          <input value={name} onChange={(event) => setName(event.target.value)} placeholder="UnionFind" />
        </Field>
        <Field label="Visibility">
          <select value={visibility} onChange={(event) => setVisibility(event.target.value as Visibility)}>
            <option value="private">private</option>
            <option value="public">public</option>
          </select>
        </Field>
      </div>
      <MarkdownField
        label="説明"
        hint="Markdown と TeX の数式（$a^2$、$$\\sum_{i} a_i$$）が使えます。2000 文字まで。"
        value={description}
        onChange={setDescription}
      />
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
        <Link className="btn btn-ghost" href={namespacePath(namespaceSlug)}>
          キャンセル
        </Link>
      </div>
      <ErrorNote error={error} />
    </Card>
  );
}
