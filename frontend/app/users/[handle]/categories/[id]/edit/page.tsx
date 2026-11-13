"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { use, useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { MarkdownField } from "@/components/MarkdownField";
import { OwnerOnly, useUserCategory } from "@/components/OwnerOnly";
import { useAsyncAction } from "@/components/useAsyncAction";
import { Breadcrumbs, Button, Card, ErrorNote, Field } from "@/components/ui";
import { categoryPath, userPath } from "@/lib/routes";
import type { Category } from "@/lib/types";

/**
 * Renames or describes a category. There is nothing else to manage: a category
 * comes with the first library filed under it and goes with the last one.
 */
export default function EditCategoryPage({ params }: { params: Promise<{ handle: string; id: string }> }) {
  const { handle, id } = use(params);
  const category = useUserCategory(handle, id);

  return (
    <div className="page">
      {category.data ? (
        <Breadcrumbs
          items={[
            { label: `@${handle}`, href: userPath(handle) },
            { label: category.data.name, href: categoryPath(handle, id) },
            { label: "編集" },
          ]}
        />
      ) : null}
      <h1>カテゴリの設定</h1>
      <OwnerOnly resource={category}>{(data) => <CategorySettingsForm category={data} />}</OwnerOnly>
    </div>
  );
}

function CategorySettingsForm({ category }: { category: Category }) {
  const { client } = useAuth();
  const router = useRouter();
  const { busy, error, run } = useAsyncAction();
  const [name, setName] = useState(category.name);
  const [description, setDescription] = useState(category.description);
  const backHref = categoryPath(category.owner, category.id);

  return (
    <Card
      title={category.name}
      owner
      note="名前を変えても URL は変わりません。Library が 1 つもなくなると、カテゴリは自動で削除されます。"
    >
      <Field label="名前" hint="1〜64 文字">
        <input value={name} onChange={(event) => setName(event.target.value)} />
      </Field>
      <MarkdownField
        label="説明"
        hint="Markdown と TeX の数式が使えます。500 文字まで。"
        value={description}
        onChange={setDescription}
      />
      <div className="row">
        <Button
          variant="primary"
          onClick={() =>
            run(async (signal) => {
              await client.withSignal(signal).updateCategory(category.id, { name, description });
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
