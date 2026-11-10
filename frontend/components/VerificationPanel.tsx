"use client";

import { isValidSubmissionUrl } from "@/lib/verification";
import { useWorkspace } from "./workspace";
import { useAsyncAction } from "./useAsyncAction";
import { Button, Card, ErrorNote, Field, ResultView } from "./ui";

/**
 * Shows the self-declared "related submission" URL links for the selected
 * Library Version. A link is NOT a verification/judge result: there is no
 * adapter, Problem selection, Submission, Judge run, verdict, score, status,
 * case result, or polling. The URL is never fetched or judged. Reading follows
 * the library's current visibility (public = anonymous readable, private =
 * 404 for everyone except the owner) and adding a link requires a session.
 */
export function VerificationPanel() {
  const { client, state, effectiveVersion, patchData } = useWorkspace();
  const { busy, error, run } = useAsyncAction();

  const { namespaceSlug, librarySlug } = state;
  const { submissionUrlInput, links, currentLink } = state.data;

  const refreshLinks = async (
    signal: AbortSignal,
  ): Promise<void> => {
    if (effectiveVersion === null) {
      throw new Error("先に Version を作成または一覧から選択してください。");
    }
    const api = client.withSignal(signal);
    patchData({
      links: await api.listVerifications(
        namespaceSlug,
        librarySlug,
        effectiveVersion,
      ),
    });
  };

  return (
    <Card
      title="関連する提出URL（提出リンク）"
      note={
        <>
          表示されるのは、提出者が自己宣言した「関連する提出」の URL です。
          URL の取得・内容確認・判定は行わず、提出者・判定結果・使用 Library Version の
          真正性も保証しません（<strong>検証済みではありません</strong>）。AC などの
          検証済みバッジは出しません。公開 Library のリンクは匿名で読め、private
          Library のリンクは所有者以外には 404 です。URL の追加にはログインが必要で、
          公開 Library は認証済みなら誰でも、private Library は所有者だけが追加できます。
        </>
      }
    >
      <Field
        label="関連する提出 URL（絶対 HTTP(S) URL）"
        hint="前後の空白は除去されます。ホストあり・制御文字なし・2048 バイト以内。URL は取得・判定されません。"
      >
        <input
          type="url"
          inputMode="url"
          value={submissionUrlInput}
          onChange={(event) =>
            patchData({ submissionUrlInput: event.target.value })
          }
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
                throw new Error(
                  "絶対 HTTP(S) URL（ホストあり）を入力してください。",
                );
              }
              if (effectiveVersion === null) {
                throw new Error(
                  "先に Version を作成または一覧から選択してください。",
                );
              }
              const api = client.withSignal(signal);
              const created = await api.createVerification(
                namespaceSlug,
                librarySlug,
                effectiveVersion,
                { submission_url: submissionUrlInput.trim() },
              );
              patchData({
                currentLink: created,
                links: await api.listVerifications(
                  namespaceSlug,
                  librarySlug,
                  effectiveVersion,
                ),
              });
            })
          }
          disabled={busy}
        >
          関連する提出URLを添付（{effectiveVersion ?? "Version 未選択"}）
        </Button>
        <Button
          onClick={() => run(async (signal) => refreshLinks(signal))}
          disabled={busy}
        >
          リンク一覧を取得
        </Button>
      </div>

      {links ? (
        <div className="list-block">
          <span className="result-label">
            関連する提出URL 一覧（自己宣言・未検証／クリックで詳細）
          </span>
          {links.length === 0 ? (
            <p className="muted">この Version に登録された提出 URL はまだありません。</p>
          ) : (
            <ul className="list">
              {links.map((link) => (
                <li key={link.id}>
                  <button
                    type="button"
                    className="list-item"
                    disabled={busy}
                    onClick={() =>
                      run(async (signal) => {
                        const detail = await client
                          .withSignal(signal)
                          .getVerification(link.id);
                        patchData({ currentLink: detail });
                      })
                    }
                  >
                    <span className="mono">#{link.id}</span>
                    {/* The URL is untrusted, self-declared text: render as text,
                        never as HTML, and never auto-fetch / navigate to it. */}
                    <span className="mono">{link.submission_url}</span>
                    <span className="mono muted">
                      {link.library_slug} v{link.version_number}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : null}

      {currentLink ? (
        <div className="detail-block">
          <span className="result-label">
            提出リンク #{currentLink.id}（自己宣言・未検証／検証結果ではありません）
          </span>
          <div className="kv">
            <span>提出 URL</span>
            <span className="mono">{currentLink.submission_url}</span>
            <span>Version</span>
            <span className="mono">
              {currentLink.library_slug} v{currentLink.version_number}
            </span>
            <span>登録者</span>
            <span className="mono">{currentLink.created_by}</span>
            <span>登録日時</span>
            <span className="mono">{currentLink.created_at}</span>
            <span>Namespace</span>
            <span className="mono">{currentLink.namespace_slug}</span>
            <span>Version ID</span>
            <span className="mono">{currentLink.library_version_id}</span>
          </div>
          <ResultView data={currentLink} label="提出リンク詳細 (data)" />
        </div>
      ) : null}
      <ErrorNote error={error} />
    </Card>
  );
}
