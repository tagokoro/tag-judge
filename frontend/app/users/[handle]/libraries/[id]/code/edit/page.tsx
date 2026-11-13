"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { use, useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { CodeEditor } from "@/components/CodeEditor";
import { LibraryEditFrame } from "@/components/LibraryEditFrame";
import { useAsyncAction } from "@/components/useAsyncAction";
import { useLatestVersion } from "@/components/useLatestVersion";
import { Button, Card, ErrorNote, LoadState } from "@/components/ui";
import type { Library } from "@/lib/types";

const SOURCE_TEMPLATE = "def add(a, b):\n    return a + b\n";

export default function EditCodePage({ params }: { params: Promise<{ handle: string; id: string }> }) {
  const { handle, id } = use(params);
  return (
    <LibraryEditFrame handle={handle} libraryId={id} title="コードを更新" crumb="コードを更新">
      {(library, backHref) => <EditCodeLoader library={library} backHref={backHref} />}
    </LibraryEditFrame>
  );
}

/** Starts the editor from the current code, or from a template when there is none. */
function EditCodeLoader(props: { library: Library; backHref: string }) {
  const latest = useLatestVersion(props.library.id);
  if (latest.data) {
    return <EditCodeForm {...props} initialSource={latest.data.source_code} />;
  }
  if (latest.empty) {
    return <EditCodeForm {...props} initialSource={SOURCE_TEMPLATE} />;
  }
  return <LoadState loading={latest.loading} error={latest.error} notFound="この Library は見つかりません。" />;
}

function EditCodeForm({
  library,
  backHref,
  initialSource,
}: {
  library: Library;
  backHref: string;
  initialSource: string;
}) {
  const { client } = useAuth();
  const router = useRouter();
  const { busy, error, run } = useAsyncAction();
  const [source, setSource] = useState(initialSource);

  return (
    <Card
      title={library.name}
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
              await client.withSignal(signal).createVersion(library.id, source);
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
