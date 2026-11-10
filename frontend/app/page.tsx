"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { Field } from "@/components/ui";
import { GOOGLE_LOGIN_PATH } from "@/lib/api";
import { ME_PATH, namespacePath } from "@/lib/routes";

export default function HomePage() {
  const { actor } = useAuth();
  const router = useRouter();
  const [slug, setSlug] = useState("");

  return (
    <div className="page home">
      <section className="hero">
        <div>
          <h1>
            ライブラリを、
            <br />
            版ごとに残す。
          </h1>
          <p>
            競技プログラミングで使うライブラリを Version として公開します。公開した Version
            のコードは書き換えられないので、どの提出でどの版を使ったかをあとから示せます。
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
        </div>
        <SampleSheet />
      </section>

      <section className="section">
        <h2>使い方</h2>
        <ol className="steps">
          <li>
            <strong>Namespace を作る</strong>
            ライブラリを置く自分の場所です。1 人 1 つ作れます。
          </li>
          <li>
            <strong>Library を作る</strong>
            公開か非公開かを選べます。非公開の Library は自分にしか見えません。
          </li>
          <li>
            <strong>Version を追加する</strong>
            ソースを登録すると v1、v2… と版が増えます。登録した版は変更できません。
          </li>
          <li>
            <strong>提出 URL を添える</strong>
            その版を使った提出の URL を 1 件記録できます。内容の確認や判定はしません。
          </li>
        </ol>
      </section>

      <section className="section">
        <h2>Namespace を開く</h2>
        <p className="note">公開されている Library は、ログインしなくても見られます。</p>
        <form
          className="jump"
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
      </section>
    </div>
  );
}

/** A static example of one library: its versions on the version line. */
function SampleSheet() {
  return (
    <figure className="sheet" aria-label="Library の表示例">
      <div className="sheet-title">
        <strong>UnionFind</strong>
        <span className="badge badge-public">公開</span>
      </div>
      <ol className="version-line">
        {[
          { n: 3, note: "経路圧縮を追加", url: true },
          { n: 2, note: "union by size", url: false },
          { n: 1, note: "最初の版", url: false },
        ].map((version) => (
          <li key={version.n}>
            <div className="version-node">
              <span className="version-dot">v{version.n}</span>
              <span className="version-meta">
                <span>{version.note}</span>
                {version.url ? <span className="chip">提出 URL あり</span> : null}
              </span>
            </div>
          </li>
        ))}
      </ol>
      <pre>{`def find(x):
    while parent[x] != x:
        parent[x] = parent[parent[x]]
        x = parent[x]
    return x`}</pre>
      <p className="sheet-url">https://atcoder.jp/contests/abc177/submissions/…</p>
    </figure>
  );
}
