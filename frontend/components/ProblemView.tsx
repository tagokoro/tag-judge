"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useRef, useState, type RefObject } from "react";
import { GOOGLE_LOGIN_PATH, isApiRequestError } from "@/lib/api";
import { draftKey, isRetryable, submitErrorMessage } from "@/lib/judging";
import { problemPath, submissionPath, userPath } from "@/lib/routes";
import type { ProblemDetail, ProblemSample } from "@/lib/types";
import { useAuth } from "./AuthProvider";
import { CodeEditor } from "./CodeEditor";
import { SubmissionTable } from "./lists";
import { RichText } from "./RichText";
import { useAsyncAction } from "./useAsyncAction";
import { useResource } from "./useResource";
import { Button, Card, ErrorNote, LoadState, VisibilityBadge } from "./ui";

type ProblemTab = "statement" | "submit";

const TABS: { tab: ProblemTab; label: string }[] = [
  { tab: "statement", label: "問題文" },
  { tab: "submit", label: "提出" },
];

/**
 * The problem page body: the statement and the submission form, as tabs.
 * `initial` is what the server fetched with the visitor's cookies; without it
 * (a draft seen through dev header auth) the problem loads here.
 *
 * The tab lives only in the URL (?tab=submit), so it survives the remount an
 * actor change causes.
 */
export function ProblemView({ problemId, initial }: { problemId: number; initial?: ProblemDetail }) {
  const problem = useResource(`problem:${problemId}`, (api) => api.getProblem(problemId), initial && { data: initial });
  const tab: ProblemTab = useSearchParams().get("tab") === "submit" ? "submit" : "statement";
  // Held here, not in the form, so it outlives a switch to the statement tab
  // and back: a retry after a lost response replays the first submission
  // instead of judging the code twice.
  const idempotencyKeyRef = useRef<string | null>(null);

  if (!problem.data) {
    // Only reached without server data, i.e. through dev header auth (the
    // server already answered 404 for everyone else). A hidden draft reads
    // exactly like a missing problem, and a later actor switch loads again.
    return (
      <div className="page">
        <LoadState loading={problem.loading} error={problem.error} notFound="この問題は見つかりません。" />
      </div>
    );
  }

  const selectTab = (next: ProblemTab) => {
    // No history entry per switch; useSearchParams follows replaceState.
    window.history.replaceState(null, "", next === "submit" ? problemPath(problemId, "submit") : problemPath(problemId));
  };

  return (
    <div className="page page-card">
      <div className="page-head">
        <div className="page-title">
          <h1>{problem.data.title}</h1>
          {problem.data.visibility !== "public" ? <VisibilityBadge visibility="private" /> : null}
        </div>
        <p className="page-sub">
          {problem.data.time_limit_ms / 1000} 秒 / {Math.round(problem.data.memory_limit_kb / 1024)} MB
          {problem.data.created_by ? (
            <>
              {" ・ 作成者 "}
              <Link href={userPath(problem.data.created_by)}>@{problem.data.created_by}</Link>
            </>
          ) : null}
        </p>
      </div>

      <div className="tabs page-tabs" role="tablist" aria-label="問題ページの表示">
        {TABS.map((item) => (
          <button
            key={item.tab}
            type="button"
            role="tab"
            aria-selected={tab === item.tab}
            onClick={() => selectTab(item.tab)}
          >
            {item.label}
          </button>
        ))}
      </div>

      {tab === "statement" ? <Statement problem={problem.data} /> : <SubmitPanel problem={problem.data} idempotencyKeyRef={idempotencyKeyRef} />}
    </div>
  );
}

function Statement({ problem }: { problem: ProblemDetail }) {
  return (
    <>
      {/* The statement brings its own headings, so its card has none. */}
      <section className="section" aria-label="問題文">
        <RichText source={problem.statement} />
      </section>
      {problem.samples.map((sample, index) => (
        <SampleCard key={index} sample={sample} number={index + 1} />
      ))}
    </>
  );
}

function SampleCard({ sample, number }: { sample: ProblemSample; number: number }) {
  return (
    <Card title={`サンプル ${number}`}>
      <div className="sample-grid">
        <SampleText label="入力" text={sample.input} />
        <SampleText label="出力" text={sample.output} />
      </div>
    </Card>
  );
}

function SampleText({ label, text }: { label: string; text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="sample">
      <div className="field-head">
        <span className="field-label">{label}</span>
        <button
          type="button"
          className="btn btn-ghost"
          aria-label={`${label}をコピー`}
          onClick={() => {
            // Missing outside secure contexts (plain http on a LAN address).
            navigator.clipboard?.writeText(text).then(
              () => setCopied(true),
              () => setCopied(false),
            );
          }}
        >
          {copied ? "コピーしました" : "コピー"}
        </button>
      </div>
      {/* Sample text is untrusted: shown as text, never as HTML. */}
      <pre className="source-view sample-text">{text}</pre>
    </div>
  );
}

function SubmitPanel({
  problem,
  idempotencyKeyRef,
}: {
  problem: ProblemDetail;
  idempotencyKeyRef: RefObject<string | null>;
}) {
  const { actor, state } = useAuth();
  if (!state.cookieChecked) {
    return <p className="muted">読み込み中…</p>;
  }
  if (!actor) {
    return (
      <p className="empty">
        提出するにはログインしてください。 <a href={GOOGLE_LOGIN_PATH}>Google でログイン</a>
      </p>
    );
  }
  return (
    <>
      <SubmitForm problem={problem} idempotencyKeyRef={idempotencyKeyRef} />
      <MySubmissions problemId={problem.id} />
    </>
  );
}

function readDraft(problemId: number): string {
  try {
    return window.localStorage.getItem(draftKey(problemId)) ?? "";
  } catch {
    return "";
  }
}

function writeDraft(problemId: number, source: string) {
  try {
    window.localStorage.setItem(draftKey(problemId), source);
  } catch {
    // A draft is a convenience; the editor works without storage.
  }
}

/** Only mounted in the browser after the session check, so the draft can be read on first render. */
function SubmitForm({
  problem,
  idempotencyKeyRef,
}: {
  problem: ProblemDetail;
  idempotencyKeyRef: RefObject<string | null>;
}) {
  const { client } = useAuth();
  const router = useRouter();
  const { busy, error, run } = useAsyncAction();
  const [source, setSource] = useState(() => readDraft(problem.id));
  // Set once the submission is created: the button stays disabled while the
  // page moves to it, so a second click cannot submit the code again.
  const [submitted, setSubmitted] = useState(false);

  const submit = () =>
    run(async (signal) => {
      const key = idempotencyKeyRef.current ?? crypto.randomUUID();
      idempotencyKeyRef.current = key;
      try {
        const submission = await client
          .withSignal(signal)
          .createSubmission(problem.id, { language: "python", source_code: source }, key);
        idempotencyKeyRef.current = null;
        setSubmitted(true);
        router.push(submissionPath(submission.id));
      } catch (caught) {
        if (!isApiRequestError(caught)) {
          throw caught;
        }
        // A refusal makes the next attempt a new submission; after a 5xx the
        // submission may exist, so the retry keeps the key to replay it.
        if (!isRetryable(caught)) {
          idempotencyKeyRef.current = null;
        }
        const message = submitErrorMessage(caught);
        throw message ? new Error(message) : caught;
      }
    });

  return (
    <Card title="提出">
      <label className="field">
        <span className="field-label">言語</span>
        <select value="python" disabled>
          <option value="python">Python</option>
        </select>
      </label>
      <div className="field">
        <span className="field-label">コード</span>
        <CodeEditor
          value={source}
          onChange={(next) => {
            setSource(next);
            writeDraft(problem.id, next);
            idempotencyKeyRef.current = null;
          }}
          label="コード（Python）"
        />
      </div>
      <div className="row">
        <Button variant="primary" onClick={submit} disabled={busy || submitted || source.trim() === ""}>
          {busy || submitted ? "提出中…" : "提出"}
        </Button>
      </div>
      <ErrorNote error={error} />
    </Card>
  );
}

function MySubmissions({ problemId }: { problemId: number }) {
  const submissions = useResource(`problem:${problemId}:mine`, (api) => api.listMySubmissions(problemId));
  return (
    <Card title="この問題への自分の提出">
      {submissions.data ? (
        submissions.data.length === 0 ? (
          <p className="empty">まだ提出していません。</p>
        ) : (
          <SubmissionTable submissions={submissions.data} showProblem={false} />
        )
      ) : (
        <LoadState loading={submissions.loading} error={submissions.error} notFound="" />
      )}
    </Card>
  );
}
