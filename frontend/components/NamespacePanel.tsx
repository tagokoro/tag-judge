"use client";

import { useState } from "react";
import { DEFAULT_NAMESPACE_SLUG } from "@/lib/verification";
import type { Namespace } from "@/lib/types";
import { useWorkspace } from "./workspace";
import { useAsyncAction } from "./useAsyncAction";
import { Button, Card, ErrorNote, Field, ResultView } from "./ui";

export function NamespacePanel() {
  const { client, namespaceSlug, setNamespaceSlug } = useWorkspace();
  const { busy, error, run } = useAsyncAction();
  const [displayName, setDisplayName] = useState("Seed Author");
  const [result, setResult] = useState<Namespace | null>(null);

  return (
    <Card
      title="Namespace"
      note="ユーザーは Namespace を 1 つだけ持てます。slug は作成時に指定し、その後は変更しません（§5.2）。"
    >
      <div className="row">
        <Field label="slug" hint="2-32 文字 / 小文字英数字とハイフン">
          <input
            value={namespaceSlug}
            onChange={(event) => setNamespaceSlug(event.target.value)}
            spellCheck={false}
            placeholder={DEFAULT_NAMESPACE_SLUG}
          />
        </Field>
        <Field label="表示名">
          <input value={displayName} onChange={(event) => setDisplayName(event.target.value)} />
        </Field>
      </div>
      <div className="row">
        <Button
          variant="primary"
          onClick={() =>
            run(async () => {
              const created = await client.createNamespace({
                slug: namespaceSlug,
                display_name: displayName,
              });
              setResult(created);
              setNamespaceSlug(created.slug);
            })
          }
          disabled={busy}
        >
          Namespace 作成
        </Button>
        <Button
          onClick={() =>
            run(async () => {
              setResult(await client.getMyNamespace());
            })
          }
          disabled={busy}
        >
          自分の Namespace を取得
        </Button>
        <Button
          variant="ghost"
          onClick={() =>
            run(async () => {
              setResult(await client.getNamespace(namespaceSlug));
            })
          }
          disabled={busy}
        >
          公開プロフィールを取得
        </Button>
      </div>
      {result ? <ResultView data={result} label="Namespace" /> : null}
      <ErrorNote error={error} />
    </Card>
  );
}
