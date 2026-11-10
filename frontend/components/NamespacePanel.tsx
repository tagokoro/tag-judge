"use client";

import { useEffect, useRef, useState } from "react";
import { isApiRequestError } from "@/lib/api";
import { DEFAULT_NAMESPACE_SLUG } from "@/lib/submissionUrl";
import { useWorkspace } from "./workspace";
import { useAsyncAction } from "./useAsyncAction";
import { Button, Card, ErrorNote, Field, ResultView } from "./ui";

export function NamespacePanel() {
  const { client, state, setNamespaceSlug, patchData } = useWorkspace();
  const { busy, error, run } = useAsyncAction();
  const [displayName, setDisplayName] = useState("Seed Author");
  // True when the signed-in actor has no namespace yet (GET /me/namespace 404).
  const [needsNamespace, setNeedsNamespace] = useState(false);

  const { namespaceSlug } = state;
  const result = state.data.namespaceResult;
  const authenticated = state.identity !== "anon";

  // Select the actor's own namespace so a signed-in user does not act on the
  // seed default (someone else's namespace) by accident. Staying on the same
  // slug only shows the profile, so data already loaded for it is kept. A
  // Google-session user without a namespace gets an empty slug to fill in; dev
  // "switch user" keeps the current slug so the 403/404 checks still work. The
  // ref is synced in an effect so the mount-time effect below calls the latest
  // closure.
  const loadMine = useRef<(signal: AbortSignal) => Promise<void>>(async () => {});
  const cookieSession = state.session !== null;
  useEffect(() => {
    loadMine.current = async (signal) => {
      try {
        const mine = await client.withSignal(signal).getMyNamespace();
        setNeedsNamespace(false);
        if (mine.slug === namespaceSlug) {
          patchData({ namespaceResult: mine });
        } else {
          setNamespaceSlug(mine.slug, { namespaceResult: mine });
        }
      } catch (caught) {
        if (isApiRequestError(caught) && caught.status === 404) {
          setNeedsNamespace(true);
          if (cookieSession) {
            setDisplayName("");
            setNamespaceSlug("", {});
          }
          return;
        }
        throw caught;
      }
    };
  }, [client, namespaceSlug, cookieSession, setNamespaceSlug, patchData]);

  // Panels remount on every actor change (keyed by authGeneration), so this
  // runs once per signed-in actor.
  useEffect(() => {
    if (!authenticated) {
      return;
    }
    void run((signal) => loadMine.current(signal));
  }, [authenticated, run]);

  return (
    <Card
      title="Namespace"
      note="ユーザーは Namespace を 1 つだけ持てます。slug は作成時に指定し、その後は変更しません（§5.2）。"
    >
      <div className="row">
        <Field label="slug" hint="2-32 文字 / 小文字英数字とハイフン">
          <input
            value={namespaceSlug}
            onChange={(event) => setNamespaceSlug(event.target.value)}
            spellCheck={false}
            placeholder={DEFAULT_NAMESPACE_SLUG}
          />
        </Field>
        <Field label="表示名">
          <input value={displayName} onChange={(event) => setDisplayName(event.target.value)} />
        </Field>
      </div>
      <div className="row">
        <Button
          variant="primary"
          onClick={() =>
            run(async (signal) => {
              const api = client.withSignal(signal);
              const created = await api.createNamespace({
                slug: namespaceSlug,
                display_name: displayName,
              });
              // Atomic: select the created namespace AND show its profile,
              // clearing data fetched for the previous namespace first.
              setNeedsNamespace(false);
              setNamespaceSlug(created.slug, { namespaceResult: created });
            })
          }
          disabled={busy}
        >
          Namespace 作成
        </Button>
        <Button
          onClick={() => run((signal) => loadMine.current(signal))}
          disabled={busy}
        >
          自分の Namespace を取得
        </Button>
        <Button
          variant="ghost"
          onClick={() =>
            run(async (signal) => {
              patchData({
                namespaceResult: await client.withSignal(signal).getNamespace(namespaceSlug),
              });
            })
          }
          disabled={busy}
        >
          公開プロフィールを取得
        </Button>
      </div>
      {needsNamespace && !result ? (
        <p className="muted">
          まだ自分の Namespace がありません。slug と表示名を入力して作成してください。他のユーザーの
          Namespace では Library の作成や更新はできません（403）。
        </p>
      ) : null}
      {result ? <ResultView data={result} label="Namespace" /> : null}
      <ErrorNote error={error} />
    </Card>
  );
}
