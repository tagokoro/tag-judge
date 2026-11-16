import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ProblemView } from "@/components/ProblemView";
import { parseIdSegment } from "@/lib/routes";
import { getProblem } from "@/lib/serverApi";
import { DEV_AUTH_AVAILABLE } from "@/lib/session";

type Params = Promise<{ id: string }>;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const id = parseIdSegment((await params).id);
  const problem = id === null ? null : await getProblem(id);
  return { title: problem ? problem.title : "問題が見つかりません" };
}

export default async function ProblemPage({ params }: { params: Params }) {
  const id = parseIdSegment((await params).id);
  if (id === null) {
    notFound();
  }
  const problem = await getProblem(id);
  if (!problem) {
    // A private problem may still be visible to a dev-header actor, which the
    // server cannot see; let the browser try. Production has no dev auth.
    if (DEV_AUTH_AVAILABLE) {
      return <ProblemView problemId={id} />;
    }
    notFound();
  }

  return <ProblemView problemId={id} initial={problem} />;
}
