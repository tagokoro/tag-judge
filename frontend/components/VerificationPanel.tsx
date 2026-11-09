"use client";

import { useCallback, useEffect, useRef } from "react";
import {
  caseResultsOf,
  formatCaseResult,
  isPollTokenCurrent,
  isTerminalStatus,
  isVerificationDetail,
  nextPollToken,
  pollVerification,
} from "@/lib/verification";
import type { VerificationView } from "@/lib/workspaceState";
import { useWorkspace } from "./workspace";
import { useAsyncAction } from "./useAsyncAction";
import { Button, Card, ErrorNote, Field, ResultView, StatusPill } from "./ui";

export function VerificationPanel() {
  const {
    client,
    state,
    effectiveVersion,
    patchData,
    selectVerification,
  } = useWorkspace();
  const { busy, error, run } = useAsyncAction();

  const { namespaceSlug, librarySlug } = state;
  const {
    problemId,
    problems,
    adapterSource,
    currentVerification,
    verifications,
  } = state.data;

  const mountedRef = useRef(true);
  const pollTokenRef = useRef(0);

  // Invalidate polling on EVERY selection (including a terminal one) and on
  // unmount (which also happens on an actor/auth change via the remount key).
  const invalidatePolling = useCallback(() => {
    pollTokenRef.current = nextPollToken(pollTokenRef.current);
  }, []);

  useEffect(
    () => () => {
      mountedRef.current = false;
      invalidatePolling();
    },
    [invalidatePolling],
  );

  // Poll GET /api/v1/library-verifications/{id} every 1s, at most 30 times,
  // until a terminal status (MVP §11.5). The token guards both the updates and
  // shouldContinue, so a superseded poll can never overwrite a newer result.
  const pollFrom = useCallback(
    async (initial: VerificationView, signal?: AbortSignal): Promise<void> => {
      invalidatePolling();
      const token = pollTokenRef.current;
      selectVerification(initial);
      const api = signal ? client.withSignal(signal) : client;
      await pollVerification<VerificationView>(
        () => api.getVerification(initial.id),
        initial,
        {
          onUpdate: (value) => {
            if (mountedRef.current && isPollTokenCurrent(token, pollTokenRef.current)) {
              patchData({ currentVerification: value });
            }
          },
          shouldContinue: () =>
            mountedRef.current && isPollTokenCurrent(token, pollTokenRef.current),
        },
      );
    },
    [client, patchData, selectVerification, invalidatePolling],
  );

  const selectedProblem = problems?.find((problem) => problem.id === problemId) ?? null;
  const caseResults = currentVerification ? caseResultsOf(currentVerification) : [];

  return (
    <Card
      title="検証 (Verification)"
      note="既存 Problem を選び、Library Version のコードと問題別 adapter を結合して既存の Submission / Judge で検証します。adapter は Version の一部ではなく verify ごとに渡します。検証結果と adapter は開始者（と admin）だけが読め、hidden ケースの内容は返されません（§5.7 / §10.4）。"
    >
      <div className="row">
        <Field label="problem_id">
          <input
            type="number"
            min={1}
            value={problemId}
            onChange={(event) => patchData({ problemId: Number(event.target.value) })}
          />
        </Field>
        <Button
          onClick={() =>
            run(async (signal) => {
              patchData({ problems: await client.withSignal(signal).listProblems() });
            })
          }
          disabled={busy}
        >
          公開 Problem 一覧を取得
        </Button>
      </div>
      {problems ? (
        <Field label="既存 Problem を選択">
          <select
            value={String(problemId)}
            onChange={(event) => patchData({ problemId: Number(event.target.value) })}
          >
            {problems.map((problem) => (
              <option key={problem.id} value={problem.id}>
                #{problem.id} {problem.title}
              </option>
            ))}
          </select>
        </Field>
      ) : null}
      {selectedProblem ? (
        <div className="detail-block">
          <span className="result-label">
            選択中: #{selectedProblem.id} {selectedProblem.title}（{selectedProblem.time_limit_ms} ms）
          </span>
          {/* Problem statement is untrusted text: rendered as text, never as HTML. */}
          <pre className="result statement-view">{selectedProblem.statement}</pre>
        </div>
      ) : null}

      <Field label="adapter_source（問題別・検証ごとに送信）">
        <textarea
          className="code-input"
          value={adapterSource}
          onChange={(event) => patchData({ adapterSource: event.target.value })}
          spellCheck={false}
        />
      </Field>
      <div className="row">
        <Button
          variant="primary"
          onClick={() =>
            run(async (signal) => {
              // Invalidate any in-flight poll BEFORE starting a new verification.
              invalidatePolling();
              if (effectiveVersion === null) {
                throw new Error("先に Version を作成または一覧から選択してください。");
              }
              const api = client.withSignal(signal);
              const created = await api.createVerification(
                namespaceSlug,
                librarySlug,
                effectiveVersion,
                { problem_id: problemId, adapter_source: adapterSource },
              );
              await pollFrom(created, signal);
            })
          }
          disabled={busy}
        >
          検証を開始（{effectiveVersion ?? "Version 未選択"}）
        </Button>
        <Button
          onClick={() =>
            run(async (signal) => {
              patchData({ verifications: await client.withSignal(signal).listVerifications() });
            })
          }
          disabled={busy}
        >
          自分の検証一覧
        </Button>
      </div>

      {verifications ? (
        <div className="list-block">
          <span className="result-label">自分の検証一覧（クリックで詳細）</span>
          {verifications.length === 0 ? (
            <p className="muted">検証はまだありません。</p>
          ) : (
            <ul className="list">
              {verifications.map((item) => (
                <li key={item.id}>
                  <button
                    type="button"
                    className="list-item"
                    disabled={busy}
                    onClick={() =>
                      run(async (signal) => {
                        // ALWAYS invalidate, even when selecting a terminal
                        // result: an older poll must not overwrite this detail.
                        invalidatePolling();
                        const api = client.withSignal(signal);
                        const detail = await api.getVerification(item.id);
                        selectVerification(detail);
                        if (!isTerminalStatus(detail.status)) {
                          await pollFrom(detail, signal);
                        }
                      })
                    }
                  >
                    <StatusPill status={item.status} />
                    <span className="mono">#{item.id}</span>
                    <span>score {item.score}</span>
                    <span className="mono muted">
                      {item.library_slug} v{item.version_number}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : null}

      {currentVerification ? (
        <div className="detail-block">
          <span className="result-label">検証 #{currentVerification.id}</span>
          <div className="kv">
            <span>ステータス</span>
            <StatusPill status={currentVerification.status} />
            <span>スコア</span>
            <strong>{currentVerification.score} / 100</strong>
            <span>Problem / Version</span>
            <span className="mono">
              #{currentVerification.problem_id} / v{currentVerification.version_number}
            </span>
            <span>Submission ID</span>
            <span className="mono">{currentVerification.submission_id}</span>
            <span>判定日時</span>
            <span>{currentVerification.judged_at ?? "—"}</span>
          </div>
          {busy && !isTerminalStatus(currentVerification.status) ? (
            <p className="muted">判定完了までポーリング中…（1 秒間隔 / 最大 30 回）</p>
          ) : null}
          {isVerificationDetail(currentVerification) ? (
            <>
              <span className="result-label">adapter_source（自分の検証のみ・テキスト表示）</span>
              <pre className="result source-view">{currentVerification.adapter_source}</pre>
            </>
          ) : null}
          <span className="result-label">ケース別結果（{caseResults.length} 件）</span>
          {caseResults.length === 0 ? (
            <p className="muted">
              pending / running の間は空配列です。hidden ケースの入力・期待出力・stdout・stderr は返されません。
            </p>
          ) : (
            <ul className="list">
              {caseResults.map((result) => (
                <li key={result.test_case_id} className="mono">
                  {formatCaseResult(result)}
                </li>
              ))}
            </ul>
          )}
          <ResultView data={currentVerification} label="検証詳細 (data)" />
        </div>
      ) : null}
      <ErrorNote error={error} />
    </Card>
  );
}
