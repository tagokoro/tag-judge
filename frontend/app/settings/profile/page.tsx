"use client";

import Link from "next/link";
import { redirect } from "next/navigation";
import { useAuth } from "@/components/AuthProvider";
import { LoginRequired } from "@/components/LoginRequired";
import { useMe } from "@/components/useMe";
import { Card, LoadState } from "@/components/ui";
import { SIGNUP_PATH, userPath } from "@/lib/routes";

/** The profile settings for a registered user; anyone without a handle registers first. */
export default function ProfileSettingsPage() {
  const { actor } = useAuth();
  const me = useMe();

  if (me.data && !me.data.handle) {
    redirect(SIGNUP_PATH);
  }
  if (!actor) {
    return <LoginRequired title="プロフィール" />;
  }
  const handle = me.data?.handle;
  return (
    <div className="page">
      <h1>プロフィール</h1>
      {handle ? (
        <Card title="ハンドル" note="ハンドルは URL に使われるので変更できません。">
          <dl className="kv">
            <dt>ハンドル</dt>
            <dd>@{handle}</dd>
          </dl>
          <div className="row">
            <Link className="btn" href={userPath(handle)}>
              プロフィールを見る
            </Link>
          </div>
        </Card>
      ) : (
        <LoadState loading={me.loading} error={me.error} notFound="" />
      )}
    </div>
  );
}
