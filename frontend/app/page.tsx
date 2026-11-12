import type { Metadata } from "next";
import { LibraryList } from "@/components/lists";
import { serverClient } from "@/lib/serverApi";

// The root page sits beside the root layout, so the layout's title template does not apply.
export const metadata: Metadata = { title: { absolute: "Libraries | TagJudge" } };

export default async function LibrariesPage() {
  const libraries = await (await serverClient()).listPublicLibraries();

  return (
    <div className="page">
      <div className="page-head">
        <h1>Libraries</h1>
        <p className="page-sub">public な Library です。新しく作られたものから順に並びます。</p>
      </div>
      {libraries.length === 0 ? (
        <p className="empty">public な Library はまだありません。</p>
      ) : (
        <div className="panel">
          <LibraryList libraries={libraries} showNamespace />
        </div>
      )}
    </div>
  );
}
