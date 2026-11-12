"use client";

import { useEffect } from "react";

/** Any error a page throws while rendering (an API outage, a bug) lands here instead of a blank screen. */
export default function ErrorPage({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <div className="page">
      <h1>ページを表示できませんでした</h1>
      <p className="error" role="alert">
        時間をおいてもう一度試してください。
        {error.digest ? `（エラー ID: ${error.digest}）` : null}
      </p>
      <div className="row">
        <button type="button" className="btn btn-primary" onClick={reset}>
          もう一度読み込む
        </button>
      </div>
    </div>
  );
}
