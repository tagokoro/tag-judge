"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { use, useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { OwnerOnly } from "@/components/OwnerOnly";
import { useAsyncAction } from "@/components/useAsyncAction";
import { fetchLatestVersionNumber, useLatestVersion } from "@/components/useLatestVersion";
import { Breadcrumbs, Button, Card, ErrorNote, Field, LoadState } from "@/components/ui";
import { HOME_PATH, libraryPath, namespacePath } from "@/lib/routes";
import { isValidSubmissionUrl } from "@/lib/submissionUrl";
import type { LibraryVersion } from "@/lib/types";

export default function EditSubmissionUrlPage({
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
          { label: "提出 URL を編集" },
        ]}
      />
      <h1>提出 URL を編集</h1>
      <OwnerOnly namespaceSlug={namespaceSlug} backHref={backHref}>
        <SubmissionUrlLoader namespaceSlug={namespaceSlug} librarySlug={librarySlug} backHref={backHref} />
      </OwnerOnly>
    </div>
  );
}

function SubmissionUrlLoader(props: { namespaceSlug: string; librarySlug: string; backHref: string }) {
  const latest = useLatestVersion(props.namespaceSlug, props.librarySlug);
  if (latest.data) {
    return <SubmissionUrlForm {...props} version={latest.data} />;
  }
  if (latest.empty) {
    return <p className="empty">先に「コードを更新」からコードを登録してください。</p>;
  }
  return <LoadState loading={latest.loading} error={latest.error} notFound="この Library は見つかりません。" />;
}

function SubmissionUrlForm({
  namespaceSlug,
  librarySlug,
  version,
  backHref,
}: {
  namespaceSlug: string;
  librarySlug: string;
  version: LibraryVersion;
  backHref: string;
}) {
  const { client } = useAuth();
  const router = useRouter();
  const { busy, error, run } = useAsyncAction();
  const [input, setInput] = useState(version.submission_url ?? "");

  // The URL belongs to the code shown when this page opened. If the code was
  // updated since (e.g. in another tab), writing would attach the URL to an
  // older, hidden upload, so stop and ask for a reload instead.
  const ensureStillLatest = async (signal: AbortSignal) => {
    const latest = await fetchLatestVersionNumber(client.withSignal(signal), namespaceSlug, librarySlug);
    if (latest !== version.version_number) {
      throw new Error("このページを開いたあとにコードが更新されました。ページを再読み込みしてから保存してください。");
    }
  };

  return (
    <Card
      title={librarySlug}
      owner
      note="今のコードを使った提出の URL を 1 件登録できます。提出の内容や判定結果は確認しません。"
    >
      <Field label="提出 URL" hint="絶対 HTTP(S) URL・2048 バイトまで">
        <input
          type="url"
          inputMode="url"
          value={input}
          onChange={(event) => setInput(event.target.value)}
          spellCheck={false}
          placeholder="https://atcoder.jp/contests/abc001/submissions/1"
        />
      </Field>
      <div className="row">
        <Button
          variant="primary"
          onClick={() =>
            run(async (signal) => {
              if (!isValidSubmissionUrl(input)) {
                throw new Error("絶対 HTTP(S) URL（ホストあり）を入力してください。");
              }
              await ensureStillLatest(signal);
              await client
                .withSignal(signal)
                .setSubmissionUrl(namespaceSlug, librarySlug, version.version_number, {
                  submission_url: input.trim(),
                });
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
        {version.submission_url !== null ? (
          <Button
            variant="danger"
            onClick={() =>
              run(async (signal) => {
                await ensureStillLatest(signal);
                await client
                  .withSignal(signal)
                  .clearSubmissionUrl(namespaceSlug, librarySlug, version.version_number);
                router.push(backHref);
              })
            }
            disabled={busy}
          >
            提出 URL を削除
          </Button>
        ) : null}
      </div>
      <ErrorNote error={error} />
    </Card>
  );
}
