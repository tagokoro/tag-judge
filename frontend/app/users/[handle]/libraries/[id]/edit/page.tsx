"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { use, useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { CategoryField } from "@/components/CategoryField";
import { LibraryEditFrame } from "@/components/LibraryEditFrame";
import { MarkdownField } from "@/components/MarkdownField";
import { useAsyncAction } from "@/components/useAsyncAction";
import { useResource } from "@/components/useResource";
import { Button, Card, ErrorNote, Field, LoadState } from "@/components/ui";
import type { Library, Category, UpdateLibraryInput, Visibility } from "@/lib/types";

export default function EditLibraryPage({ params }: { params: Promise<{ handle: string; id: string }> }) {
  const { handle, id } = use(params);
  return (
    <LibraryEditFrame handle={handle} libraryId={id} title="Library の設定" crumb="設定を編集">
      {(library, backHref) => <LibrarySettingsLoader library={library} backHref={backHref} />}
    </LibraryEditFrame>
  );
}

function LibrarySettingsLoader({ library, backHref }: { library: Library; backHref: string }) {
  const categories = useResource("me/categories", (api) => api.listMyCategories());
  if (!categories.data) {
    return <LoadState loading={categories.loading} error={categories.error} notFound="" />;
  }
  return <LibrarySettingsForm library={library} categories={categories.data} backHref={backHref} />;
}

function LibrarySettingsForm({
  library,
  categories,
  backHref,
}: {
  library: Library;
  categories: Category[];
  backHref: string;
}) {
  const { client } = useAuth();
  const router = useRouter();
  const { busy, error, run } = useAsyncAction();
  const [name, setName] = useState(library.name);
  const [description, setDescription] = useState(library.description);
  const [visibility, setVisibility] = useState<Visibility>(library.visibility);
  const [category, setCategory] = useState(library.category.name);

  return (
    <Card
      title={library.name}
      owner
      note="private にすると、自分以外からはコードも見えなくなります。カテゴリを変えても、この Library の URL は変わりません。移したあとに空になったカテゴリは自動で削除されます。"
    >
      <div className="row row-fields">
        <Field label="名前" hint="1〜80 文字">
          <input value={name} onChange={(event) => setName(event.target.value)} />
        </Field>
        <Field label="Visibility">
          <select value={visibility} onChange={(event) => setVisibility(event.target.value as Visibility)}>
            <option value="private">private</option>
            <option value="public">public</option>
          </select>
        </Field>
      </div>
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
              const patch: UpdateLibraryInput = { name, description, visibility };
              if (category.trim().toLowerCase() !== library.category.name.toLowerCase()) {
                patch.category = category;
              }
              await client.withSignal(signal).updateLibrary(library.id, patch);
              router.push(backHref);
            })
          }
          disabled={busy}
        >
          保存
        </Button>
        <Link className="btn btn-ghost" href={backHref}>
          キャンセル
        </Link>
      </div>
      <ErrorNote error={error} />
    </Card>
  );
}
