"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { use, useState } from "react";
import { useAuth } from "@/components/AuthProvider";
import { useAsyncAction } from "@/components/useAsyncAction";
import { useResource } from "@/components/useResource";
import {
  Breadcrumbs,
  Button,
  Card,
  ErrorNote,
  Field,
  LoadState,
  VisibilityBadge,
} from "@/components/ui";
import { HOME_PATH, namespacePath, versionPath } from "@/lib/routes";
import { isOwner } from "@/lib/session";
import type { Library, Visibility } from "@/lib/types";

const SOURCE_TEMPLATE = "def add(a, b):\n    return a + b\n";

export default function LibraryPage({
  params,
}: {
  params: Promise<{ namespace: string; library: string }>;
}) {
  const { namespace: namespaceSlug, library: librarySlug } = use(params);
  const { actor } = useAuth();
  const namespace = useResource(`ns:${namespaceSlug}`, (api) => api.getNamespace(namespaceSlug));
  const library = useResource(`lib:${namespaceSlug}/${librarySlug}`, (api) =>
    api.getLibrary(namespaceSlug, librarySlug),
  );
  const versions = useResource(`lib:${namespaceSlug}/${librarySlug}:versions`, (api) =>
    api.listVersions(namespaceSlug, librarySlug),
  );

  const owner = namespace.data !== null && isOwner(actor, namespace.data.owner_user_id);

  return (
    <div className="page">
      <Breadcrumbs
        items={[
          { label: "ホーム", href: HOME_PATH },
          { label: namespaceSlug, href: namespacePath(namespaceSlug) },
          { label: librarySlug },
        ]}
      />
      {library.data ? (
        <>
          <div className="page-title">
            <h1>{library.data.name}</h1>
            <VisibilityBadge visibility={library.data.visibility} />
          </div>
          {library.data.description ? (
            <p className="text-block">{library.data.description}</p>
          ) : null}
        </>
      ) : (
        <LoadState
          loading={library.loading}
          error={library.error}
          notFound="この Library は見つかりません（private の Library は所有者にしか見えません）。"
        />
      )}

      {library.data ? (
        <Card title="Version">
          {versions.data ? (
            versions.data.length === 0 ? (
              <p className="muted">Version はまだありません。</p>
            ) : (
              <ul className="list">
                {versions.data.map((version) => (
                  <li key={version.id}>
                    <Link
                      className="list-item"
                      href={versionPath(namespaceSlug, librarySlug, version.version_number)}
                    >
                      <strong className="mono">v{version.version_number}</strong>
                      <span className="muted">{version.created_at}</span>
                      <span className="muted">{version.byte_size} bytes</span>
                      {version.submission_url ? <span className="pill">提出 URL あり</span> : null}
                    </Link>
                  </li>
                ))}
              </ul>
            )
          ) : (
            <LoadState loading={versions.loading} error={versions.error} notFound="" />
          )}
        </Card>
      ) : null}

      {owner && library.data ? (
        <>
          <CreateVersionForm namespaceSlug={namespaceSlug} librarySlug={librarySlug} />
          <LibrarySettings
            namespaceSlug={namespaceSlug}
            library={library.data}
            onSaved={library.setData}
          />
        </>
      ) : null}
    </div>
  );
}

function CreateVersionForm({
  namespaceSlug,
  librarySlug,
}: {
  namespaceSlug: string;
  librarySlug: string;
}) {
  const { client } = useAuth();
  const router = useRouter();
  const { busy, error, run } = useAsyncAction();
  const [source, setSource] = useState(SOURCE_TEMPLATE);

  return (
    <Card
      title="Version を追加"
      note="Python のソース 1 ファイルです（100,000 バイトまで）。作成した Version のソースは変更・削除できません。"
    >
      <Field label="ソース">
        <textarea
          className="code-input"
          value={source}
          onChange={(event) => setSource(event.target.value)}
          spellCheck={false}
        />
      </Field>
      <div className="row">
        <Button
          variant="primary"
          onClick={() =>
            run(async (signal) => {
              const created = await client
                .withSignal(signal)
                .createVersion(namespaceSlug, librarySlug, source);
              router.push(versionPath(namespaceSlug, librarySlug, created.version_number));
            })
          }
          disabled={busy}
        >
          Version を作成
        </Button>
      </div>
      <ErrorNote error={error} />
    </Card>
  );
}

function LibrarySettings({
  namespaceSlug,
  library,
  onSaved,
}: {
  namespaceSlug: string;
  library: Library;
  onSaved: (library: Library) => void;
}) {
  const { client } = useAuth();
  const { busy, error, run } = useAsyncAction();
  const [name, setName] = useState(library.name);
  const [description, setDescription] = useState(library.description);
  const [visibility, setVisibility] = useState<Visibility>(library.visibility);

  return (
    <Card title="Library の設定" note="private にすると、所有者以外からは Library も Version も見えなくなります。">
      <div className="row">
        <Field label="名前">
          <input value={name} onChange={(event) => setName(event.target.value)} />
        </Field>
        <Field label="公開範囲">
          <select
            value={visibility}
            onChange={(event) => setVisibility(event.target.value as Visibility)}
          >
            <option value="private">private（自分だけ）</option>
            <option value="public">public（誰でも閲覧可）</option>
          </select>
        </Field>
      </div>
      <Field label="説明">
        <textarea value={description} onChange={(event) => setDescription(event.target.value)} />
      </Field>
      <div className="row">
        <Button
          onClick={() =>
            run(async (signal) => {
              onSaved(
                await client
                  .withSignal(signal)
                  .updateLibrary(namespaceSlug, library.slug, { name, description, visibility }),
              );
            })
          }
          disabled={busy}
        >
          保存
        </Button>
      </div>
      <ErrorNote error={error} />
    </Card>
  );
}
