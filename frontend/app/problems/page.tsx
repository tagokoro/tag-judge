import type { Metadata } from "next";
import { ProblemList } from "@/components/lists";
import { serverClient } from "@/lib/serverApi";

export const metadata: Metadata = { title: "Problems" };

export default async function ProblemsPage() {
  const problems = await (await serverClient()).listProblems();

  return (
    <div className="page">
      <h1>Problems</h1>
      {problems.length === 0 ? (
        <p className="empty">public な問題はまだありません。</p>
      ) : (
        <div className="panel">
          <ProblemList problems={problems} />
        </div>
      )}
    </div>
  );
}
