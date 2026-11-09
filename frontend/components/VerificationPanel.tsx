"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  caseResultsOf,
  DEFAULT_ADAPTER_SOURCE,
  DEFAULT_PROBLEM_ID,
  formatCaseResult,
  isTerminalStatus,
  isVerificationDetail,
  pollVerification,
} from "@/lib/verification";
import type {
  Problem,
  VerificationDetail,
  VerificationSummary,
} from "@/lib/types";
import { useWorkspace } from "./workspace";
import { useAsyncAction } from "./useAsyncAction";
import { Button, Card, ErrorNote, Field, ResultView, StatusPill } from "./ui";

type VerificationView = VerificationSummary | VerificationDetail;

export function VerificationPanel() {
  const { client, namespaceSlug, librarySlug, versionNumber } = useWorkspace();
  const { busy, error, run } = useAsyncAction();

  const [problemId, setProblemId] = useState<number>(DEFAULT_PROBLEM_ID);
  const [problems, setProblems] = useState<Problem[] | null>(null);
  const [adapter, setAdapter] = useState<string>(DEFAULT_ADAPTER_SOURCE);
  const [current, setCurrent] = useState<VerificationView | null>(null);
  const [verifications, setVerifications] = useState<VerificationSummary[] | null>(null);

  const mountedRef = useRef(true);
  const pollTokenRef = useRef(0);
  useEffect(
    () => () => {
      mountedRef.current = false;
      pollTokenRef.current += 1;
    },
    [],
  );

  // Poll GET /api/v1/library-verifications/{id} every 1s, at most 30 times,
  // until a terminal status (MVP §11.5). A token cancels stale polls on unmount
  // or when a new verification is started.
  const pollFrom = useCallback(
    async (initial: VerificationView): Promise<void> => {
      const token = (pollTokenRef.current += 1);
      setCurrent(initial);
      await pollVerification<VerificationView>(
        () => client.getVerification(initial.id),
        initial,
        {
          onUpdate: (value) => {
            if (mountedRef.current && pollTokenRef.current === token) {
              setCurrent(value);
            }
          },
          shouldContinue: () => mountedRef.current && pollTokenRef.current === token,
        },
      );
    },
    [client],
  );

  const selectedProblem = problems?.find((problem) => problem.id === problemId) ?? null;
  const caseResults = current ? caseResultsOf(current) : [];

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
            onChange={(event) => setProblemId(Number(event.target.value))}
          />
        </Field>
        <Button
          onClick={() =>
            run(async () => {
              setProblems(await client.listProblems());
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
            onChange={(event) => setProblemId(Number(event.target.value))}
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
          value={adapter}
          onChange={(event) => setAdapter(event.target.value)}
          spellCheck={false}
        />
      </Field>
      <div className="row">
        <Button
          variant="primary"
          onClick={() =>
            run(async () => {
              if (versionNumber === null) {
                throw new Error("先に Version を作成または一覧から選択してください。");
              }
              const created = await client.createVerification(
                namespaceSlug,
                librarySlug,
                versionNumber,
                { problem_id: problemId, adapter_source: adapter },
              );
              await pollFrom(created);
            })
          }
          disabled={busy}
        >
          検証を開始（{versionNumber ?? "Version 未選択"}）
        </Button>
        <Button
          onClick={() =>
            run(async () => {
              setVerifications(await client.listVerifications());
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
                      run(async () => {
                        const detail = await client.getVerification(item.id);
                        setCurrent(detail);
                        if (!isTerminalStatus(detail.status)) {
                          await pollFrom(detail);
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

      {current ? (
        <div className="detail-block">
          <span className="result-label">検証 #{current.id}</span>
          <div className="kv">
            <span>ステータス</span>
            <StatusPill status={current.status} />
            <span>スコア</span>
            <strong>{current.score} / 100</strong>
            <span>Problem / Version</span>
            <span className="mono">
              #{current.problem_id} / v{current.version_number}
            </span>
            <span>Submission ID</span>
            <span className="mono">{current.submission_id}</span>
            <span>判定日時</span>
            <span>{current.judged_at ?? "—"}</span>
          </div>
          {busy && !isTerminalStatus(current.status) ? (
            <p className="muted">判定完了までポーリング中…（1 秒間隔 / 最大 30 回）</p>
          ) : null}
          {isVerificationDetail(current) ? (
            <>
              <span className="result-label">adapter_source（自分の検証のみ・テキスト表示）</span>
              <pre className="result source-view">{current.adapter_source}</pre>
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
          <ResultView data={current} label="検証詳細 (data)" />
        </div>
      ) : null}
      <ErrorNote error={error} />
    </Card>
  );
}
