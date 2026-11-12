"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { use, useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { CodeEditor } from "@/components/CodeEditor";
import { OwnerOnly } from "@/components/OwnerOnly";
import { useAsyncAction } from "@/components/useAsyncAction";
import { useLatestVersion } from "@/components/useLatestVersion";
import { Breadcrumbs, Button, Card, ErrorNote, LoadState } from "@/components/ui";
import { HOME_PATH, libraryPath, namespacePath } from "@/lib/routes";

const SOURCE_TEMPLATE = "def add(a, b):\n    return a + b\n";

export default function EditCodePage({
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
          { label: "Libraries", href: HOME_PATH },
          { label: namespaceSlug, href: namespacePath(namespaceSlug) },
          { label: librarySlug, href: backHref },
          { label: "コードを更新" },
        ]}
      />
      <h1>コードを更新</h1>
      <OwnerOnly namespaceSlug={namespaceSlug} backHref={backHref}>
        <EditCodeLoader namespaceSlug={namespaceSlug} librarySlug={librarySlug} backHref={backHref} />
      </OwnerOnly>
    </div>
  );
}

/** Starts the editor from the current code, or from a template when there is none. */
function EditCodeLoader(props: { namespaceSlug: string; librarySlug: string; backHref: string }) {
  const latest = useLatestVersion(props.namespaceSlug, props.librarySlug);
  if (latest.data) {
    return <EditCodeForm {...props} initialSource={latest.data.source_code} />;
  }
  if (latest.empty) {
    return <EditCodeForm {...props} initialSource={SOURCE_TEMPLATE} />;
  }
  return <LoadState loading={latest.loading} error={latest.error} notFound="この Library は見つかりません。" />;
}

function EditCodeForm({
  namespaceSlug,
  librarySlug,
  backHref,
  initialSource,
}: {
  namespaceSlug: string;
  librarySlug: string;
  backHref: string;
  initialSource: string;
}) {
  const { client } = useAuth();
  const router = useRouter();
  const { busy, error, run } = useAsyncAction();
  const [source, setSource] = useState(initialSource);

  return (
    <Card
      title={librarySlug}
      owner
      note="Python のソース 1 ファイル（100,000 バイトまで）を保存します。保存すると提出 URL は空になるので、必要なら登録し直してください。"
    >
      <div className="field">
        <span className="field-label">コード（Python）</span>
        <CodeEditor value={source} onChange={setSource} label="コード（Python）" />
      </div>
      <div className="row">
        <Button
          variant="primary"
          onClick={() =>
            run(async (signal) => {
              await client.withSignal(signal).createVersion(namespaceSlug, librarySlug, source);
              router.push(backHref);
            })
          }
          // Saving unchanged code would only clear the submission URL.
          disabled={busy || source === initialSource}
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
