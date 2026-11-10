"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { use, useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { OwnerOnly } from "@/components/OwnerOnly";
import { MarkdownField } from "@/components/MarkdownField";
import { useAsyncAction } from "@/components/useAsyncAction";
import { useResource } from "@/components/useResource";
import { Breadcrumbs, Button, Card, ErrorNote, Field, LoadState } from "@/components/ui";
import { HOME_PATH, libraryPath, namespacePath } from "@/lib/routes";
import type { Library, Visibility } from "@/lib/types";

export default function EditLibraryPage({
  params,
}: {
  params: Promise<{ namespace: string; library: string }>;
}) {
  const { namespace: namespaceSlug, library: librarySlug } = use(params);
  const backHref = libraryPath(namespaceSlug, librarySlug);
  const library = useResource(`lib:${namespaceSlug}/${librarySlug}`, (api) =>
    api.getLibrary(namespaceSlug, librarySlug),
  );

  return (
    <div className="page">
      <Breadcrumbs
        items={[
          { label: "Libraries", href: HOME_PATH },
          { label: namespaceSlug, href: namespacePath(namespaceSlug) },
          { label: librarySlug, href: backHref },
          { label: "設定を編集" },
        ]}
      />
      <h1>Library の設定</h1>
      <OwnerOnly namespaceSlug={namespaceSlug} backHref={backHref}>
        {library.data ? (
          <LibrarySettingsForm namespaceSlug={namespaceSlug} library={library.data} backHref={backHref} />
        ) : (
          <LoadState loading={library.loading} error={library.error} notFound="この Library は見つかりません。" />
        )}
      </OwnerOnly>
    </div>
  );
}

function LibrarySettingsForm({
  namespaceSlug,
  library,
  backHref,
}: {
  namespaceSlug: string;
  library: Library;
  backHref: string;
}) {
  const { client } = useAuth();
  const router = useRouter();
  const { busy, error, run } = useAsyncAction();
  const [name, setName] = useState(library.name);
  const [description, setDescription] = useState(library.description);
  const [visibility, setVisibility] = useState<Visibility>(library.visibility);

  return (
    <Card
      title={library.name}
      owner
      note="private にすると、自分以外からは Library も Version も見えなくなります。slug は変更できません。"
    >
      <div className="row">
        <Field label="名前">
          <input value={name} onChange={(event) => setName(event.target.value)} />
        </Field>
        <Field label="Visibility">
          <select value={visibility} onChange={(event) => setVisibility(event.target.value as Visibility)}>
            <option value="private">private（自分だけ）</option>
            <option value="public">public（誰でも見られる）</option>
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
              await client
                .withSignal(signal)
                .updateLibrary(namespaceSlug, library.slug, { name, description, visibility });
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
