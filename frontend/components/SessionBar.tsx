"use client";

import { useState } from "react";
import { GOOGLE_LOGIN_PATH } from "@/lib/api";
import { SEED_USERS } from "@/lib/verification";
import type { Me, Role } from "@/lib/types";
import { useWorkspace } from "./workspace";
import { useAsyncAction } from "./useAsyncAction";
import { Button, Card, ErrorNote, Field, ResultView } from "./ui";

export function SessionBar() {
  const {
    client,
    state,
    setDevEnabled,
    setDevUserId,
    setDevRole,
    switchDevUser,
    refreshSession,
    logout,
  } = useWorkspace();
  const { busy, error, run } = useAsyncAction();
  const [meResult, setMeResult] = useState<Me | null>(null);

  const { session, cookieChecked, devEnabled, devUser } = state;

  return (
    <Card
      title="認証 / セッション"
      note="cookie セッションが優先されます。Google ログインは同一オリジンの /api/v1/auth/google/login へのリンクです。"
    >
      {!cookieChecked ? (
        <p className="muted">セッションを確認しています…</p>
      ) : session ? (
        <div className="session-indicator">
          <span className="pill">ログイン中（cookie セッション）</span>
          <span>ユーザー: {session.user_id}</span>
          <span>ロール: {session.role}</span>
          <Button
            onClick={() =>
              run(async () => {
                await logout();
                setMeResult(null);
              })
            }
            disabled={busy}
          >
            ログアウト
          </Button>
        </div>
      ) : (
        <div className="stack">
          <p>未ログインです。Google ログイン、またはローカル開発専用の dev 認証を使ってください。</p>
          {/* A plain same-origin link (not fetch): the browser follows the 302 to Google. */}
          <a className="btn btn-primary link-button" href={GOOGLE_LOGIN_PATH}>
            Google でログイン
          </a>
        </div>
      )}

      {cookieChecked && !session ? (
        <div className="dev-auth">
          <label className="checkbox">
            <input
              type="checkbox"
              checked={devEnabled}
              onChange={(event) => setDevEnabled(event.target.checked)}
            />
            dev ヘッダー認証を使う（ローカル開発専用・明示的なオプトイン）
          </label>
          {devEnabled ? (
            <div className="stack">
              <div className="warn" role="note">
                ⚠ これは <code>DEV_HEADER_AUTH=true</code> のローカル環境専用です。本番では絶対に有効にしないでください。チェックを入れたときだけ <code>X-User-ID</code> / <code>X-User-Role</code> を送信します。cookie セッションがある場合は送信しません。
              </div>
              <div className="row">
                <Field label="ユーザーID">
                  <input
                    value={devUser.userId}
                    onChange={(event) => setDevUserId(event.target.value)}
                    spellCheck={false}
                  />
                </Field>
                <Field label="ロール">
                  <select
                    value={devUser.role}
                    onChange={(event) => setDevRole(event.target.value as Role)}
                  >
                    <option value="author">author</option>
                    <option value="admin">admin</option>
                    <option value="user">user</option>
                  </select>
                </Field>
                <Button onClick={switchDevUser}>ユーザー切替</Button>
              </div>
              <p className="pill">
                dev 認証中: {devUser.userId} / {devUser.role} （切替先: {SEED_USERS[1].userId}）
              </p>
            </div>
          ) : null}
        </div>
      ) : null}

      <div className="row">
        <Button
          onClick={() =>
            run(async (signal) => {
              setMeResult(await client.withSignal(signal).getMe());
            })
          }
          disabled={busy}
        >
          GET /me で現在の actor を確認
        </Button>
        <Button
          variant="ghost"
          onClick={() =>
            run(async () => {
              await refreshSession();
              setMeResult(null);
            })
          }
          disabled={busy}
        >
          セッション再確認
        </Button>
      </div>
      {meResult ? <ResultView data={meResult} label="GET /me" /> : null}
      <ErrorNote error={error} />
    </Card>
  );
}
