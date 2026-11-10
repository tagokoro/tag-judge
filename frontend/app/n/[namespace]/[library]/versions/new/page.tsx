"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { use, useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { OwnerOnly } from "@/components/OwnerOnly";
import { useAsyncAction } from "@/components/useAsyncAction";
import { Breadcrumbs, Button, Card, ErrorNote, Field } from "@/components/ui";
import { HOME_PATH, libraryPath, namespacePath, versionPath } from "@/lib/routes";

const SOURCE_TEMPLATE = "def add(a, b):\n    return a + b\n";

export default function NewVersionPage({
  params,
}: {
  params: Promise<{ namespace: string; library: string }>;
}) {
  const { namespace: namespaceSlug, library: librarySlug } = use(params);
  const backHref = libraryPath(namespaceSlug, librarySlug);

  return (
    <div className="page">
      <Breadcrumbs
        items={[
          { label: "ライブラリ一覧", href: HOME_PATH },
          { label: namespaceSlug, href: namespacePath(namespaceSlug) },
          { label: librarySlug, href: backHref },
          { label: "Version を追加" },
        ]}
      />
      <h1>Version を追加</h1>
      <OwnerOnly namespaceSlug={namespaceSlug} backHref={backHref}>
        <NewVersionForm namespaceSlug={namespaceSlug} librarySlug={librarySlug} backHref={backHref} />
      </OwnerOnly>
    </div>
  );
}

function NewVersionForm({
  namespaceSlug,
  librarySlug,
  backHref,
}: {
  namespaceSlug: string;
  librarySlug: string;
  backHref: string;
}) {
  const { client } = useAuth();
  const router = useRouter();
  const { busy, error, run } = useAsyncAction();
  const [source, setSource] = useState(SOURCE_TEMPLATE);

  return (
    <Card
      title={librarySlug}
      owner
      note="Python のソース 1 ファイル（100,000 バイトまで）を登録します。登録した Version は変更も削除もできません。"
    >
      <Field label="ソース">
        <textarea
          className="code-input"
          value={source}
          onChange={(event) => setSource(event.target.value)}
          spellCheck={false}
        />
      </Field>
      <div className="row">
        <Button
          variant="primary"
          onClick={() =>
            run(async (signal) => {
              const created = await client
                .withSignal(signal)
                .createVersion(namespaceSlug, librarySlug, source);
              router.push(versionPath(namespaceSlug, librarySlug, created.version_number));
            })
          }
          disabled={busy}
        >
          Version を作成
        </Button>
        <Link className="btn btn-ghost" href={backHref}>
          キャンセル
        </Link>
      </div>
      <ErrorNote error={error} />
    </Card>
  );
}
