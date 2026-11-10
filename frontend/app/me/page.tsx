"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { LoginRequired } from "@/components/LoginRequired";
import { useAsyncAction } from "@/components/useAsyncAction";
import { useResource } from "@/components/useResource";
import { Button, Card, ErrorNote, Field, LoadState } from "@/components/ui";
import { isApiRequestError } from "@/lib/api";
import { namespacePath } from "@/lib/routes";

/**
 * 自分のライブラリ: the user's libraries live on their namespace page, so this
 * page forwards there, or asks them to create the namespace first.
 */
export default function MyLibrariesPage() {
  const { actor } = useAuth();
  const router = useRouter();
  const mine = useResource(actor ? "me/namespace" : null, (api) => api.getMyNamespace());

  const slug = mine.data?.slug;
  useEffect(() => {
    if (slug) {
      router.replace(namespacePath(slug));
    }
  }, [slug, router]);

  if (!actor) {
    return <LoginRequired title="自分のライブラリ" />;
  }

  const missing = isApiRequestError(mine.error) && mine.error.status === 404;

  return (
    <div className="page">
      <h1>自分のライブラリ</h1>
      {missing ? (
        <CreateNamespaceForm />
      ) : (
        <LoadState loading={mine.loading || mine.data !== null} error={mine.error} notFound="" />
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
      note="Library を置くには、まず自分の Namespace を作ります。1 人 1 つ作れます。slug は URL に使われ、あとから変更できません。"
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
