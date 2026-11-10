"use client";

import { LibraryList } from "@/components/lists";
import { useResource } from "@/components/useResource";
import { LoadState } from "@/components/ui";

export default function LibrariesPage() {
  const libraries = useResource("libraries", (api) => api.listPublicLibraries());

  return (
    <div className="page">
      <div className="page-head">
        <h1>ライブラリ一覧</h1>
        <p className="page-sub">公開されている Library です。新しく作られたものから順に並びます。</p>
      </div>
      {libraries.data ? (
        libraries.data.length === 0 ? (
          <p className="empty">公開されている Library はまだありません。</p>
        ) : (
          <LibraryList libraries={libraries.data} showNamespace />
        )
      ) : (
        <LoadState loading={libraries.loading} error={libraries.error} notFound="" />
      )}
    </div>
  );
}
