"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { use, useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { OwnerOnly } from "@/components/OwnerOnly";
import { useAsyncAction } from "@/components/useAsyncAction";
import { useResource } from "@/components/useResource";
import { Breadcrumbs, Button, Card, ErrorNote, Field, LoadState } from "@/components/ui";
import { HOME_PATH, libraryPath, namespacePath, parseVersionParam, versionPath } from "@/lib/routes";
import { isValidSubmissionUrl } from "@/lib/submissionUrl";
import type { LibraryVersion } from "@/lib/types";

export default function EditSubmissionUrlPage({
  params,
}: {
  params: Promise<{ namespace: string; library: string; version: string }>;
}) {
  const { namespace: namespaceSlug, library: librarySlug, version: rawVersion } = use(params);
  const versionNumber = parseVersionParam(rawVersion);
  const backHref = versionPath(namespaceSlug, librarySlug, versionNumber ?? 0);
  const version = useResource(
    versionNumber === null ? null : `ver:${namespaceSlug}/${librarySlug}/${versionNumber}`,
    (api) => api.getVersion(namespaceSlug, librarySlug, versionNumber ?? 0),
  );

  return (
    <div className="page">
      <Breadcrumbs
        items={[
          { label: "Libraries", href: HOME_PATH },
          { label: namespaceSlug, href: namespacePath(namespaceSlug) },
          { label: librarySlug, href: libraryPath(namespaceSlug, librarySlug) },
          { label: `v${rawVersion}`, href: backHref },
          { label: "提出 URL を編集" },
        ]}
      />
      <h1>提出 URL を編集</h1>
      {versionNumber === null ? (
        <p className="empty">Version 番号が正しくありません。</p>
      ) : (
        <OwnerOnly namespaceSlug={namespaceSlug} backHref={backHref}>
          {version.data ? (
            <SubmissionUrlForm
              namespaceSlug={namespaceSlug}
              librarySlug={librarySlug}
              version={version.data}
              backHref={backHref}
            />
          ) : (
            <LoadState loading={version.loading} error={version.error} notFound="この Version は見つかりません。" />
          )}
        </OwnerOnly>
      )}
    </div>
  );
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

  return (
    <Card
      title={`${librarySlug} v${version.version_number}`}
      owner
      note="この版を使った提出の URL を 1 件登録できます。提出の内容や判定結果は確認しません。"
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
