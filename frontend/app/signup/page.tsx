"use client";

import { redirect, useRouter } from "next/navigation";
import { useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { LoginRequired } from "@/components/LoginRequired";
import { useAsyncAction } from "@/components/useAsyncAction";
import { useMe } from "@/components/useMe";
import { Button, Card, ErrorNote, Field, LoadState } from "@/components/ui";
import { isApiRequestError } from "@/lib/api";
import { userPath } from "@/lib/routes";

/**
 * The sign-up page: a signed-in user without a handle types one in. The login
 * callback sends new users here. Anyone who already has a handle goes to their
 * own page.
 */
export default function SignupPage() {
  const { actor } = useAuth();
  const me = useMe();

  if (me.data?.handle) {
    redirect(userPath(me.data.handle));
  }
  if (!actor) {
    return <LoginRequired title="ユーザー登録" />;
  }
  return (
    <div className="page">
      <h1>ユーザー登録</h1>
      {me.data ? <SignupForm /> : <LoadState loading={me.loading} error={me.error} notFound="" />}
    </div>
  );
}

function SignupForm() {
  const { client, refresh } = useAuth();
  const router = useRouter();
  const { busy, error, setError, run } = useAsyncAction();
  const [handle, setHandle] = useState("");

  return (
    <Card
      title="ハンドルを決める"
      note="ハンドルは TagJudge でのあなたの名前です。ページの URL（/users/ハンドル）や、作成者・提出者の表示に使われます。公開され、あとから変更できません。"
    >
      <Field label="ハンドル" hint="2〜32 文字の英小文字・数字・ハイフン（例：tagokoro、my-name）">
        <input
          value={handle}
          onChange={(event) => setHandle(event.target.value)}
          spellCheck={false}
          autoCapitalize="none"
          autoComplete="off"
        />
      </Field>
      <div className="row">
        <Button
          variant="primary"
          onClick={() =>
            run(async (signal) => {
              try {
                const created = await client.withSignal(signal).createProfile({ handle: handle.trim() });
                await refresh();
                router.push(userPath(created.handle));
              } catch (caught) {
                if (isApiRequestError(caught) && caught.code === "HANDLE_TAKEN") {
                  setError("このハンドルは使われています。別のハンドルにしてください。");
                  return;
                }
                if (isApiRequestError(caught) && caught.code === "INVALID_HANDLE") {
                  setError("ハンドルは 2〜32 文字の英小文字・数字・ハイフンで入力してください（ハイフンは先頭・末尾・連続には使えません）。");
                  return;
                }
                throw caught;
              }
            })
          }
          disabled={busy || handle.trim() === ""}
        >
          登録
        </Button>
      </div>
      <ErrorNote error={error} />
    </Card>
  );
}
