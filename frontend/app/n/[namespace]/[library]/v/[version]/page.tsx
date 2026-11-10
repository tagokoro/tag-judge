"use client";

import { use, useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { useAsyncAction } from "@/components/useAsyncAction";
import { useResource } from "@/components/useResource";
import { Breadcrumbs, Button, Card, ErrorNote, Field, LoadState } from "@/components/ui";
import { HOME_PATH, libraryPath, namespacePath, parseVersionParam } from "@/lib/routes";
import { isOwner } from "@/lib/session";
import { isValidSubmissionUrl } from "@/lib/submissionUrl";
import type { LibraryVersion } from "@/lib/types";

export default function VersionPage({
  params,
}: {
  params: Promise<{ namespace: string; library: string; version: string }>;
}) {
  const { namespace: namespaceSlug, library: librarySlug, version: rawVersion } = use(params);
  const versionNumber = parseVersionParam(rawVersion);
  const { actor } = useAuth();
  const namespace = useResource(`ns:${namespaceSlug}`, (api) => api.getNamespace(namespaceSlug));
  const version = useResource(
    versionNumber === null ? null : `ver:${namespaceSlug}/${librarySlug}/${versionNumber}`,
    (api) => api.getVersion(namespaceSlug, librarySlug, versionNumber ?? 0),
  );

  const owner = namespace.data !== null && isOwner(actor, namespace.data.owner_user_id);

  return (
    <div className="page">
      <Breadcrumbs
        items={[
          { label: "ホーム", href: HOME_PATH },
          { label: namespaceSlug, href: namespacePath(namespaceSlug) },
          { label: librarySlug, href: libraryPath(namespaceSlug, librarySlug) },
          { label: `v${rawVersion}` },
        ]}
      />
      {versionNumber === null ? (
        <p className="note">Version 番号が正しくありません。</p>
      ) : version.data ? (
        <>
          <div className="page-title">
            <h1>
              {librarySlug} <span className="mono">v{version.data.version_number}</span>
            </h1>
          </div>
          <Card title="ソース">
            <div className="kv">
              <span>作成日時</span>
              <span className="mono">{version.data.created_at}</span>
              <span>サイズ</span>
              <span className="mono">{version.data.byte_size} bytes</span>
              <span>SHA-256</span>
              <span className="mono hash">{version.data.content_sha256}</span>
            </div>
            {/* Source is untrusted text: rendered as a text node, never as HTML. */}
            <pre className="source-view">{version.data.source_code}</pre>
          </Card>
          <SubmissionUrlCard
            namespaceSlug={namespaceSlug}
            librarySlug={librarySlug}
            version={version.data}
            owner={owner}
            onChange={version.setData}
          />
        </>
      ) : (
        <LoadState
          loading={version.loading}
          error={version.error}
          notFound="この Version は見つかりません（private の Library は所有者にしか見えません）。"
        />
      )}
    </div>
  );
}

function SubmissionUrlCard({
  namespaceSlug,
  librarySlug,
  version,
  owner,
  onChange,
}: {
  namespaceSlug: string;
  librarySlug: string;
  version: LibraryVersion;
  owner: boolean;
  onChange: (version: LibraryVersion) => void;
}) {
  const { client } = useAuth();
  const { busy, error, run } = useAsyncAction();
  const [input, setInput] = useState(version.submission_url ?? "");

  return (
    <Card
      title="関連する提出 URL"
      note="所有者が自己申告した、このライブラリを使った提出の URL です。URL の内容や判定結果は確認していません。"
    >
      <div className="kv">
        <span>URL</span>
        {/* Self-declared, untrusted text: shown as text, never fetched or followed automatically. */}
        <span className="mono">{version.submission_url ?? "未設定"}</span>
        <span>更新日時</span>
        <span className="mono">{version.submission_url_updated_at ?? "—"}</span>
      </div>

      {owner ? (
        <>
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
                  onChange(
                    await client
                      .withSignal(signal)
                      .setSubmissionUrl(namespaceSlug, librarySlug, version.version_number, {
                        submission_url: input.trim(),
                      }),
                  );
                })
              }
              disabled={busy}
            >
              保存
            </Button>
            <Button
              onClick={() =>
                run(async (signal) => {
                  const api = client.withSignal(signal);
                  await api.clearSubmissionUrl(namespaceSlug, librarySlug, version.version_number);
                  onChange(await api.getVersion(namespaceSlug, librarySlug, version.version_number));
                  setInput("");
                })
              }
              disabled={busy || version.submission_url === null}
            >
              削除
            </Button>
          </div>
          <ErrorNote error={error} />
        </>
      ) : null}
    </Card>
  );
}
