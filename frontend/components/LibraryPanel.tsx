"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { DEFAULT_LIBRARY_SLUG } from "@/lib/verification";
import type { Visibility } from "@/lib/types";
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
    state,
    setLibrarySlug,
    patchData,
    clearLibraryDetail,
  } = useWorkspace();
  const { busy, error, run } = useAsyncAction();

  // Pure input widgets: they are reset by the authGeneration remount on an
  // actor change, so nothing identity-scoped is kept in component-local state.
  const [name, setName] = useState("UnionFind");
  const [description, setDescription] = useState("");
  const [visibility, setVisibility] = useState<Visibility>("private");

  const { namespaceSlug, librarySlug } = state;
  const { libraries, libraryDetail, patchVisibility } = state.data;

  // On a failed (e.g. 404 private) detail refetch, clear the old detail AND the
  // displayed version source instead of leaving the previous actor's data up.
  const loadDetail = useCallback(
    async (signal?: AbortSignal): Promise<void> => {
      const api = signal ? client.withSignal(signal) : client;
      try {
        const library = await api.getLibrary(namespaceSlug, librarySlug);
        patchData({ libraryDetail: library, patchVisibility: library.visibility });
      } catch (caught) {
        if (!signal?.aborted) {
          clearLibraryDetail();
        }
        throw caught;
      }
    },
    [client, namespaceSlug, librarySlug, patchData, clearLibraryDetail],
  );

  const loadList = useCallback(
    async (signal?: AbortSignal): Promise<void> => {
      const api = signal ? client.withSignal(signal) : client;
      patchData({ libraries: await api.listLibraries(namespaceSlug) });
    },
    [client, namespaceSlug, patchData],
  );

  // "switch user" re-runs the current Library detail fetch (MVP §11): private
  // libraries must 404 for other users and become 200 once made public. The ref
  // is synced in an effect so it is never read/written during render.
  const loadDetailRef = useRef(loadDetail);
  useEffect(() => {
    loadDetailRef.current = loadDetail;
  }, [loadDetail]);
  useEffect(() => {
    if (state.refreshTick === 0) {
      return;
    }
    void run(async (signal) => {
      await loadDetailRef.current(signal);
    });
  }, [state.refreshTick, run]);

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
            run(async (signal) => {
              const api = client.withSignal(signal);
              const created = await api.createLibrary(namespaceSlug, {
                slug: librarySlug,
                name,
                description,
                visibility,
              });
              const libraries = await api.listLibraries(namespaceSlug);
              // Atomically select the created library AND apply its data: the
              // reducer clears any stale links/detail/versions, then applies
              // this patch in one guarded dispatch.
              setLibrarySlug(created.slug, {
                libraryDetail: created,
                patchVisibility: created.visibility,
                libraries,
              });
            })
          }
          disabled={busy}
        >
          Library 作成
        </Button>
        <Button
          onClick={() =>
            run(async (signal) => {
              await loadList(signal);
            })
          }
          disabled={busy}
        >
          一覧を取得
        </Button>
        <Button
          variant="ghost"
          onClick={() =>
            run(async (signal) => {
              await loadDetail(signal);
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
            onChange={(event) =>
              patchData({ patchVisibility: event.target.value as Visibility })
            }
          >
            <option value="private">private</option>
            <option value="public">public</option>
          </select>
        </Field>
        <Button
          onClick={() =>
            run(async (signal) => {
              const api = client.withSignal(signal);
              const updated = await api.updateLibrary(namespaceSlug, librarySlug, {
                visibility: patchVisibility,
              });
              patchData({
                libraryDetail: updated,
                patchVisibility: updated.visibility,
                libraries: await api.listLibraries(namespaceSlug),
              });
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
                    disabled={busy}
                    onClick={() =>
                      run(async (signal) => {
                        const api = client.withSignal(signal);
                        const detail = await api.getLibrary(namespaceSlug, library.slug);
                        // Atomic: switch target + show its detail, so the old
                        // library's links / version source are cleared first.
                        setLibrarySlug(library.slug, {
                          libraryDetail: detail,
                          patchVisibility: detail.visibility,
                        });
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

      {libraryDetail ? (
        <div className="detail-block">
          <span className="result-label">Library 詳細</span>
          <p>
            <VisibilityBadge visibility={libraryDetail.visibility} />{" "}
            <span className="mono">{libraryDetail.slug}</span> — {libraryDetail.name}
          </p>
          <p className="text-block">{libraryDetail.description || "（説明なし）"}</p>
          <ResultView data={libraryDetail} label="レスポンス (data)" />
        </div>
      ) : null}
      <ErrorNote error={error} />
    </Card>
  );
}
