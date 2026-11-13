"use client";

import { redirect } from "next/navigation";
import { useAuth } from "@/components/AuthProvider";
import { LoginRequired } from "@/components/LoginRequired";
import { useMe } from "@/components/useMe";
import { LoadState } from "@/components/ui";
import { SIGNUP_PATH, userPath } from "@/lib/routes";

/**
 * 自分のページ: forwards to the user's own page, or to sign-up when they have
 * no handle yet.
 */
export default function MyPage() {
  const { actor } = useAuth();
  const me = useMe();

  if (me.data) {
    redirect(me.data.handle ? userPath(me.data.handle) : SIGNUP_PATH);
  }
  if (!actor) {
    return <LoginRequired title="自分のページ" />;
  }
  return (
    <div className="page">
      <h1>自分のページ</h1>
      <LoadState loading={me.loading} error={me.error} notFound="" />
    </div>
  );
}
