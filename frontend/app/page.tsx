"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { Card, Field } from "@/components/ui";
import { GOOGLE_LOGIN_PATH } from "@/lib/api";
import { ME_PATH, namespacePath } from "@/lib/routes";

export default function HomePage() {
  const { actor } = useAuth();
  const router = useRouter();
  const [slug, setSlug] = useState("");

  return (
    <div className="page">
      <section className="hero">
        <h1>Library Sharing Judge</h1>
        <p>
          競技プログラミング用ライブラリを自分の Namespace に置き、不変な Version として公開できます。
          各 Version には、そのライブラリを使った提出の URL を 1 件添えられます。
        </p>
        <div className="row">
          {actor ? (
            <Link className="btn btn-primary" href={ME_PATH}>
              自分の Namespace を開く
            </Link>
          ) : (
            <a className="btn btn-primary" href={GOOGLE_LOGIN_PATH}>
              Google でログインして始める
            </a>
          )}
        </div>
      </section>

      <Card title="Namespace を見る" note="公開されている Library は、ログインしていなくても閲覧できます。">
        <form
          className="row"
          onSubmit={(event) => {
            event.preventDefault();
            if (slug.trim() !== "") {
              router.push(namespacePath(slug.trim()));
            }
          }}
        >
          <Field label="Namespace の slug">
            <input
              value={slug}
              onChange={(event) => setSlug(event.target.value)}
              spellCheck={false}
              placeholder="seed-author"
            />
          </Field>
          <button type="submit" className="btn">
            開く
          </button>
        </form>
      </Card>

      <Card title="使い方">
        <ol className="steps">
          <li>ログインして、自分の Namespace を作成します（1 ユーザー 1 つ）。</li>
          <li>Namespace に Library を作ります。公開範囲は public / private から選べます。</li>
          <li>Library に Version を追加します。Version のソースは後から変更できません。</li>
          <li>
            Version に、そのライブラリを使った提出の URL を設定できます。URL は自己申告で、内容の確認や判定は行いません。
          </li>
        </ol>
      </Card>
    </div>
  );
}
