"use client";

import { useState } from "react";
import { DEFAULT_LIBRARY_SOURCE } from "@/lib/verification";
import type { LibraryVersion, LibraryVersionSummary } from "@/lib/types";
import { useWorkspace } from "./workspace";
import { useAsyncAction } from "./useAsyncAction";
import { Button, Card, ErrorNote, Field, ResultView } from "./ui";

export function VersionPanel() {
  const {
    client,
    namespaceSlug,
    librarySlug,
    versionNumber,
    setVersionNumber,
  } = useWorkspace();
  const { busy, error, run } = useAsyncAction();

  const [source, setSource] = useState<string>(DEFAULT_LIBRARY_SOURCE);
  const [created, setCreated] = useState<LibraryVersion | null>(null);
  const [summaries, setSummaries] = useState<LibraryVersionSummary[] | null>(null);
  const [selected, setSelected] = useState<LibraryVersion | null>(null);

  return (
    <Card
      title="Library Version"
      note="Version は作成時点で不変です（更新・削除 API はありません / §5.1）。source_code は読める人にだけ返り、private Library は所有者以外 404 です。ソースは必ずテキストとして表示します。"
    >
      <Field label="Python ソース（1 本 / 100000 バイトまで）">
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
            run(async () => {
              const version = await client.createVersion(namespaceSlug, librarySlug, source);
              setCreated(version);
              setSelected(version);
              setVersionNumber(version.version_number);
              setSummaries(await client.listVersions(namespaceSlug, librarySlug));
            })
          }
          disabled={busy}
        >
          Version 作成
        </Button>
        <Button
          onClick={() =>
            run(async () => {
              setSummaries(await client.listVersions(namespaceSlug, librarySlug));
            })
          }
          disabled={busy}
        >
          一覧を取得
        </Button>
      </div>
      <p className="muted">
        検証対象の Version: <strong>{versionNumber ?? "未選択"}</strong>
        （一覧から選択、または作成すると自動選択）
      </p>

      {created ? <ResultView data={created} label="作成レスポンス (data)" /> : null}

      {summaries ? (
        <div className="list-block">
          <span className="result-label">Version 一覧（クリックでソースを表示）</span>
          {summaries.length === 0 ? (
            <p className="muted">Version はまだありません。</p>
          ) : (
            <ul className="list">
              {summaries.map((summary) => (
                <li key={summary.id}>
                  <button
                    type="button"
                    className="list-item"
                    onClick={() =>
                      run(async () => {
                        const version = await client.getVersion(
                          namespaceSlug,
                          librarySlug,
                          summary.version_number,
                        );
                        setSelected(version);
                        setVersionNumber(version.version_number);
                      })
                    }
                  >
                    <span className="mono">v{summary.version_number}</span>
                    <span className="mono muted">{summary.content_sha256.slice(0, 12)}…</span>
                    <span>{summary.byte_size} bytes</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : null}

      {selected ? (
        <div className="detail-block">
          <span className="result-label">
            v{selected.version_number} のソース（テキスト表示・sha256 {selected.content_sha256}）
          </span>
          <pre className="result source-view">{selected.source_code}</pre>
        </div>
      ) : null}
      <ErrorNote error={error} />
    </Card>
  );
}
