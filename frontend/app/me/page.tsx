"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { useAsyncAction } from "@/components/useAsyncAction";
import { useResource } from "@/components/useResource";
import { Button, Card, ErrorNote, Field, LoadState } from "@/components/ui";
import { GOOGLE_LOGIN_PATH, isApiRequestError } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { namespacePath } from "@/lib/routes";

export default function MyPage() {
  const { actor } = useAuth();
  const mine = useResource(actor ? "me/namespace" : null, (api) => api.getMyNamespace());

  if (!actor) {
    return (
      <div className="page">
        <h1>自分の Namespace</h1>
        <Card title="ログインが必要です">
          <p>自分の Namespace を作成・表示するにはログインしてください。</p>
          <div className="row">
            <a className="btn btn-primary" href={GOOGLE_LOGIN_PATH}>
              Google でログイン
            </a>
          </div>
        </Card>
      </div>
    );
  }

  const missing = isApiRequestError(mine.error) && mine.error.status === 404;

  return (
    <div className="page">
      <h1>自分の Namespace</h1>
      {mine.data ? (
        <Card title={mine.data.display_name}>
          <dl className="kv">
            <dt>slug</dt>
            <dd>@{mine.data.slug}</dd>
            <dt>作成日時</dt>
            <dd className="num">{formatDateTime(mine.data.created_at)}</dd>
          </dl>
          <div className="row">
            <Link className="btn btn-primary" href={namespacePath(mine.data.slug)}>
              Library 一覧を開く
            </Link>
          </div>
        </Card>
      ) : missing ? (
        <CreateNamespaceForm />
      ) : (
        <LoadState loading={mine.loading} error={mine.error} notFound="" />
      )}
    </div>
  );
}

function CreateNamespaceForm() {
  const { client } = useAuth();
  const router = useRouter();
  const { busy, error, run } = useAsyncAction();
  const [slug, setSlug] = useState("");
  const [displayName, setDisplayName] = useState("");

  return (
    <Card
      title="Namespace を作成"
      note="まだ自分の Namespace がありません。1 人 1 つ作れます。slug は URL に使われ、あとから変更できません。"
    >
      <div className="row">
        <Field label="slug" hint="2〜32 文字の英小文字・数字・ハイフン">
          <input
            value={slug}
            onChange={(event) => setSlug(event.target.value)}
            spellCheck={false}
            placeholder="my-name"
          />
        </Field>
        <Field label="表示名" hint="1〜64 文字">
          <input value={displayName} onChange={(event) => setDisplayName(event.target.value)} />
        </Field>
      </div>
      <div className="row">
        <Button
          variant="primary"
          onClick={() =>
            run(async (signal) => {
              const created = await client
                .withSignal(signal)
                .createNamespace({ slug: slug.trim(), display_name: displayName });
              router.push(namespacePath(created.slug));
            })
          }
          disabled={busy}
        >
          作成
        </Button>
      </div>
      <ErrorNote error={error} />
    </Card>
  );
}
