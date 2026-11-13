"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { use, useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { CategoryField } from "@/components/CategoryField";
import { LoginRequired } from "@/components/LoginRequired";
import { MarkdownField } from "@/components/MarkdownField";
import { useAsyncAction } from "@/components/useAsyncAction";
import { useMe } from "@/components/useMe";
import { useResource } from "@/components/useResource";
import { Button, Card, ErrorNote, Field, LoadState } from "@/components/ui";
import { categoryPath, libraryPath, SIGNUP_PATH, userPath } from "@/lib/routes";
import type { Category, Visibility } from "@/lib/types";

/**
 * Creates a library. Its category is picked or typed in the form and created
 * on the fly when new; `?category={id}` starts the form in that category.
 */
export default function NewLibraryPage({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  const requested = use(searchParams).category;
  const { actor } = useAuth();
  const me = useMe();
  const categories = useResource(actor ? "me/categories" : null, (api) => api.listMyCategories());

  if (!actor) {
    return <LoginRequired title="Library を作成" />;
  }
  const handle = me.data?.handle;
  let body;
  if (me.data && !handle) {
    body = (
      <p className="empty">
        Library を作るには、先に <Link href={SIGNUP_PATH}>ユーザー登録</Link> をしてください。
      </p>
    );
  } else if (handle && categories.data) {
    const initial = categories.data.find((category) => category.id === requested)?.name ?? "";
    body = <NewLibraryForm handle={handle} categories={categories.data} initialCategory={initial} />;
  } else {
    body = (
      <LoadState loading={me.loading || categories.loading} error={me.error ?? categories.error} notFound="" />
    );
  }
  return (
    <div className="page">
      <h1>Library を作成</h1>
      {body}
    </div>
  );
}

function NewLibraryForm({
  handle,
  categories,
  initialCategory,
}: {
  handle: string;
  categories: Category[];
  initialCategory: string;
}) {
  const { client } = useAuth();
  const router = useRouter();
  const { busy, error, run } = useAsyncAction();
  const [category, setCategory] = useState(initialCategory);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [visibility, setVisibility] = useState<Visibility>("private");
  const startedIn = categories.find((item) => item.name === initialCategory);

  return (
    <Card title="新しい Library" owner>
      <div className="row row-fields">
        <Field label="名前" hint="1〜80 文字（日本語も使えます）">
          <input value={name} onChange={(event) => setName(event.target.value)} placeholder="UnionFind" />
        </Field>
        <Field label="Visibility">
          <select value={visibility} onChange={(event) => setVisibility(event.target.value as Visibility)}>
            <option value="private">private</option>
            <option value="public">public</option>
          </select>
        </Field>
      </div>
      {/* On its own row: its hint grows when the name is new, which would
          misalign inputs that share a bottom-aligned row. */}
      <CategoryField value={category} onChange={setCategory} categories={categories} />
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
              const created = await client
                .withSignal(signal)
                .createLibrary({ name, category, description, visibility });
              router.push(libraryPath(handle, created.id));
            })
          }
          disabled={busy}
        >
          作成
        </Button>
        <Link className="btn btn-ghost" href={startedIn ? categoryPath(handle, startedIn.id) : userPath(handle)}>
          キャンセル
        </Link>
      </div>
      <ErrorNote error={error} />
    </Card>
  );
}
