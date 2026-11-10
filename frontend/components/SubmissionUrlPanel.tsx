"use client";

import { isValidSubmissionUrl } from "@/lib/submissionUrl";
import { useWorkspace } from "./workspace";
import { useAsyncAction } from "./useAsyncAction";
import { Button, Card, ErrorNote, Field } from "./ui";

/**
 * Shows and edits the self-declared "related submission" URL of the selected
 * Library Version. Each version has at most one URL, and only the library owner
 * may set or clear it; reading it follows the version's visibility (it comes
 * with the version itself).
 */
export function SubmissionUrlPanel() {
  const { client, state, effectiveVersion, patchData } = useWorkspace();
  const { busy, error, run } = useAsyncAction();

  const { namespaceSlug, librarySlug } = state;
  const { submissionUrlInput, versionSource } = state.data;
  const selected =
    versionSource !== null && versionSource.version_number === effectiveVersion
      ? versionSource
      : null;

  const requireVersion = (): number => {
    if (effectiveVersion === null || selected === null) {
      throw new Error("先に Version を作成または一覧から選択してください。");
    }
    return effectiveVersion;
  };

  return (
    <Card
      title="関連する提出URL"
      note={
        <>
          Version ごとに 1 件、Library の所有者だけが設定・変更・削除できます。表示されるのは
          所有者が自己宣言した「関連する提出」の URL です。URL の取得・内容確認・判定は行わず、
          提出者・判定結果・使用 Library Version の真正性も保証しません（
          <strong>検証済みではありません</strong>）。AC などの検証済みバッジは出しません。
          URL は Version と同じ公開範囲で読めます（private Library は所有者以外 404）。
        </>
      }
    >
      <div className="kv">
        <span>対象 Version</span>
        <span className="mono">{selected ? `v${selected.version_number}` : "未選択"}</span>
        <span>現在の提出 URL</span>
        {/* The URL is untrusted, self-declared text: render as text, never as
            HTML, and never auto-fetch / navigate to it. */}
        <span className="mono">{selected?.submission_url ?? "未設定"}</span>
        <span>更新日時</span>
        <span className="mono">{selected?.submission_url_updated_at ?? "—"}</span>
      </div>

      <Field
        label="関連する提出 URL（絶対 HTTP(S) URL）"
        hint="前後の空白は除去されます。ホストあり・制御文字なし・2048 バイト以内。URL は取得・判定されません。"
      >
        <input
          type="url"
          inputMode="url"
          value={submissionUrlInput}
          onChange={(event) => patchData({ submissionUrlInput: event.target.value })}
          spellCheck={false}
          placeholder="https://example.com/submissions/1"
        />
      </Field>
      <div className="row">
        <Button
          variant="primary"
          onClick={() =>
            run(async (signal) => {
              if (!isValidSubmissionUrl(submissionUrlInput)) {
                throw new Error("絶対 HTTP(S) URL（ホストあり）を入力してください。");
              }
              const version = requireVersion();
              const api = client.withSignal(signal);
              const updated = await api.setSubmissionUrl(namespaceSlug, librarySlug, version, {
                submission_url: submissionUrlInput.trim(),
              });
              patchData({
                versionSource: updated,
                versionSummaries: await api.listVersions(namespaceSlug, librarySlug),
              });
            })
          }
          disabled={busy}
        >
          提出URLを設定（{selected ? `v${selected.version_number}` : "Version 未選択"}）
        </Button>
        <Button
          onClick={() =>
            run(async (signal) => {
              const version = requireVersion();
              const api = client.withSignal(signal);
              await api.clearSubmissionUrl(namespaceSlug, librarySlug, version);
              patchData({
                versionSource: await api.getVersion(namespaceSlug, librarySlug, version),
                versionSummaries: await api.listVersions(namespaceSlug, librarySlug),
              });
            })
          }
          disabled={busy || selected?.submission_url == null}
        >
          提出URLを削除
        </Button>
      </div>
      <ErrorNote error={error} />
    </Card>
  );
}
