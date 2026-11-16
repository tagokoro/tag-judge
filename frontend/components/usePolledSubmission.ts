"use client";

import { useEffect, useState } from "react";
import { isApiRequestError } from "@/lib/api";
import { isJudging, isRetryable, MAX_POLL_FAILURES, pollDelay, POLL_GIVE_UP_MS } from "@/lib/judging";
import type { SubmissionDetail } from "@/lib/types";
import { useAuth } from "./AuthProvider";
import { useResource } from "./useResource";

export type PollStop = "timeout" | "error";

export interface PolledSubmission {
  data: SubmissionDetail | null;
  error: unknown;
  loading: boolean;
  /** Why polling gave up while the submission was still being judged. */
  stopped: PollStop | null;
  /** Resumes polling after it stopped, keeping what is on screen meanwhile. */
  reload: () => void;
}

/**
 * Loads a submission and, while it is pending or running, polls it until the
 * verdict arrives: 0.5 s apart at first, slowing to every 3 s. Polling pauses
 * while the tab is hidden (and polls at once when it is shown again), gives up
 * after two minutes, three failures in a row, or a 4xx, and stops on unmount. An actor
 * change remounts the page (ActorScope), which drops everything polled.
 */
export function usePolledSubmission(submissionId: number, initial?: SubmissionDetail): PolledSubmission {
  const { client } = useAuth();
  const resource = useResource(
    `submission:${submissionId}`,
    (api) => api.getSubmission(submissionId),
    initial && { data: initial },
  );
  const base = resource.data;
  // Polled results and the stop reason belong to one load; a reload starts over.
  const [polled, setPolled] = useState<{ base: SubmissionDetail; data: SubmissionDetail } | null>(null);
  const [stop, setStop] = useState<{ base: SubmissionDetail; reason: PollStop } | null>(null);

  const current = polled !== null && polled.base === base ? polled.data : base;
  const stopped = stop !== null && stop.base === base ? stop.reason : null;
  const judging = current !== null && isJudging(current.status);

  useEffect(() => {
    if (base === null || !judging || stopped !== null) {
      return;
    }
    const controller = new AbortController();
    const startedAt = Date.now();
    let timer: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;
    let failures = 0;
    let inFlight = false;

    const schedule = () => {
      timer = setTimeout(poll, pollDelay(attempt));
      attempt += 1;
    };
    const poll = async () => {
      if (inFlight || document.hidden) {
        return; // resumed by the visibilitychange listener
      }
      if (Date.now() - startedAt > POLL_GIVE_UP_MS) {
        setStop({ base, reason: "timeout" });
        return;
      }
      inFlight = true;
      try {
        const next = await client.withSignal(controller.signal).getSubmission(submissionId);
        failures = 0;
        setPolled({ base, data: next });
        if (!isJudging(next.status)) {
          return;
        }
      } catch (caught) {
        if (controller.signal.aborted) {
          return;
        }
        failures += 1;
        if (failures >= MAX_POLL_FAILURES || (isApiRequestError(caught) && !isRetryable(caught))) {
          setStop({ base, reason: "error" });
          return;
        }
      } finally {
        inFlight = false;
      }
      schedule();
    };
    const onVisibilityChange = () => {
      if (!document.hidden) {
        clearTimeout(timer);
        void poll();
      }
    };

    schedule();
    document.addEventListener("visibilitychange", onVisibilityChange);
    return () => {
      controller.abort();
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", onVisibilityChange);
    };
  }, [base, judging, stopped, client, submissionId]);

  return { data: current, error: resource.error, loading: resource.loading, stopped, reload: () => setStop(null) };
}
