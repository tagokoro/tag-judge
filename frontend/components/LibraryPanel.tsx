"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { DEFAULT_LIBRARY_SLUG } from "@/lib/verification";
import type { Library, Visibility } from "@/lib/types";
import { useWorkspace } from "./workspace";
import { useAsyncAction } from "./useAsyncAction";
import {
  Button,
  Card,
  ErrorNote,
  Field,
  ResultView,
  VisibilityBadge,
} from "./ui";

export function LibraryPanel() {
  const {
    client,
    namespaceSlug,
    librarySlug,
    setLibrarySlug,
    refreshTick,
  } = useWorkspace();
  const { busy, error, run } = useAsyncAction();

  const [name, setName] = useState("UnionFind");
  const [description, setDescription] = useState("");
  const [visibility, setVisibility] = useState<Visibility>("private");
  const [patchVisibility, setPatchVisibility] = useState<Visibility>("private");
  const [libraries, setLibraries] = useState<Library[] | null>(null);
  const [detail, setDetail] = useState<Library | null>(null);

  const showDetail = useCallback((library: Library) => {
    setDetail(library);
    setPatchVisibility(library.visibility);
  }, []);

  const loadDetail = useCallback(async () => {
    showDetail(await client.getLibrary(namespaceSlug, librarySlug));
  }, [client, namespaceSlug, librarySlug, showDetail]);

  const loadList = useCallback(async () => {
    setLibraries(await client.listLibraries(namespaceSlug));
  }, [client, namespaceSlug]);

  // "switch user" re-runs the current library detail fetch (MVP §11): private
  // libraries must 404 for other users and become 200 once made public. The ref
  // is synced in an effect so it is never read/written during render.
  const loadDetailRef = useRef(loadDetail);
  useEffect(() => {
    loadDetailRef.current = loadDetail;
  }, [loadDetail]);
  useEffect(() => {
    if (refreshTick === 0) {
      return;
    }
    void run(async () => {
      await loadDetailRef.current();
    });
  }, [refreshTick, run]);

  return (
    <Card
      title="Library"
      note="public / private の公開範囲を切り替えられます。private は所有者以外には 404 となり一覧にも表示されません（§5.6）。説明文はテキストとして表示します。"
    >
      <div className="row">
        <Field label="slug" hint=" Namespace 内で一意 / 2-48 文字">
          <input
            value={librarySlug}
            onChange={(event) => setLibrarySlug(event.target.value)}
            spellCheck={false}
            placeholder={DEFAULT_LIBRARY_SLUG}
          />
        </Field>
        <Field label="名前">
          <input value={name} onChange={(event) => setName(event.target.value)} />
        </Field>
      </div>
      <Field label="説明（テキスト表示）">
        <textarea value={description} onChange={(event) => setDescription(event.target.value)} />
      </Field>
      <div className="row">
        <Field label="公開範囲">
          <select
            value={visibility}
            onChange={(event) => setVisibility(event.target.value as Visibility)}
          >
            <option value="private">private</option>
            <option value="public">public</option>
          </select>
        </Field>
        <Button
          variant="primary"
          onClick={() =>
            run(async () => {
              const created = await client.createLibrary(namespaceSlug, {
                slug: librarySlug,
                name,
                description,
                visibility,
              });
              setLibrarySlug(created.slug);
              showDetail(created);
              await loadList();
            })
          }
          disabled={busy}
        >
          Library 作成
        </Button>
        <Button
          onClick={() =>
            run(async () => {
              await loadList();
            })
          }
          disabled={busy}
        >
          一覧を取得
        </Button>
        <Button
          variant="ghost"
          onClick={() =>
            run(async () => {
              await loadDetail();
            })
          }
          disabled={busy}
        >
          詳細を再取得
        </Button>
      </div>

      <div className="row">
        <Field label="公開範囲を更新（PATCH）">
          <select
            value={patchVisibility}
            onChange={(event) => setPatchVisibility(event.target.value as Visibility)}
          >
            <option value="private">private</option>
            <option value="public">public</option>
          </select>
        </Field>
        <Button
          onClick={() =>
            run(async () => {
              showDetail(
                await client.updateLibrary(namespaceSlug, librarySlug, {
                  visibility: patchVisibility,
                }),
              );
              await loadList();
            })
          }
          disabled={busy}
        >
          visibility を更新
        </Button>
      </div>

      {libraries ? (
        <div className="list-block">
          <span className="result-label">Library 一覧（クリックで詳細）</span>
          {libraries.length === 0 ? (
            <p className="muted">表示できる Library はありません。</p>
          ) : (
            <ul className="list">
              {libraries.map((library) => (
                <li key={library.id}>
                  <button
                    type="button"
                    className="list-item"
                    onClick={() =>
                      run(async () => {
                        setLibrarySlug(library.slug);
                        showDetail(await client.getLibrary(namespaceSlug, library.slug));
                      })
                    }
                  >
                    <VisibilityBadge visibility={library.visibility} />
                    <span className="mono">{library.slug}</span>
                    <span>{library.name}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : null}

      {detail ? (
        <div className="detail-block">
          <span className="result-label">Library 詳細</span>
          <p>
            <VisibilityBadge visibility={detail.visibility} />{" "}
            <span className="mono">{detail.slug}</span> — {detail.name}
          </p>
          <p className="text-block">{detail.description || "（説明なし）"}</p>
          <ResultView data={detail} label="レスポンス (data)" />
        </div>
      ) : null}
      <ErrorNote error={error} />
    </Card>
  );
}
