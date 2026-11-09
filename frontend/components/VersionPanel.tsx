"use client";

import { useWorkspace } from "./workspace";
import { useAsyncAction } from "./useAsyncAction";
import { Button, Card, ErrorNote, Field, ResultView } from "./ui";

export function VersionPanel() {
  const { client, state, setVersion, effectiveVersion, patchData } = useWorkspace();
  const { busy, error, run } = useAsyncAction();

  const { namespaceSlug, librarySlug } = state;
  const { versionCreated, versionSummaries, versionSource, librarySourceInput } = state.data;

  return (
    <Card
      title="Library Version"
      note="Version は作成時点で不変です（更新・削除 API はありません / §5.1）。source_code は読める人にだけ返り、private Library は所有者以外 404 です。ソースは必ずテキストとして表示します。"
    >
      <Field label="Python ソース（1 本 / 100000 バイトまで）">
        <textarea
          className="code-input"
          value={librarySourceInput}
          onChange={(event) => patchData({ librarySourceInput: event.target.value })}
          spellCheck={false}
        />
      </Field>
      <div className="row">
        <Button
          variant="primary"
          onClick={() =>
            run(async (signal) => {
              const api = client.withSignal(signal);
              const version = await api.createVersion(
                namespaceSlug,
                librarySlug,
                librarySourceInput,
              );
              patchData({
                versionCreated: version,
                versionSource: version,
                versionSummaries: await api.listVersions(namespaceSlug, librarySlug),
              });
              // Bind the selection to the CURRENT namespace/library slugs.
              setVersion(version.version_number);
            })
          }
          disabled={busy}
        >
          Version 作成
        </Button>
        <Button
          onClick={() =>
            run(async (signal) => {
              const api = client.withSignal(signal);
              patchData({
                versionSummaries: await api.listVersions(namespaceSlug, librarySlug),
              });
            })
          }
          disabled={busy}
        >
          一覧を取得
        </Button>
      </div>
      <p className="muted">
        検証対象の Version: <strong>{effectiveVersion ?? "未選択"}</strong>
        （Namespace / Library を変えると選択は無効になります）
      </p>

      {versionCreated ? (
        <ResultView data={versionCreated} label="作成レスポンス (data)" />
      ) : null}

      {versionSummaries ? (
        <div className="list-block">
          <span className="result-label">Version 一覧（クリックでソースを表示）</span>
          {versionSummaries.length === 0 ? (
            <p className="muted">Version はまだありません。</p>
          ) : (
            <ul className="list">
              {versionSummaries.map((summary) => (
                <li key={summary.id}>
                  <button
                    type="button"
                    className="list-item"
                    disabled={busy}
                    onClick={() =>
                      run(async (signal) => {
                        const api = client.withSignal(signal);
                        const version = await api.getVersion(
                          namespaceSlug,
                          librarySlug,
                          summary.version_number,
                        );
                        patchData({ versionSource: version });
                        setVersion(version.version_number);
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

      {versionSource ? (
        <div className="detail-block">
          <span className="result-label">
            v{versionSource.version_number} のソース（テキスト表示・sha256 {versionSource.content_sha256}）
          </span>
          <pre className="result source-view">{versionSource.source_code}</pre>
        </div>
      ) : null}
      <ErrorNote error={error} />
    </Card>
  );
}
