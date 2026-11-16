import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { SubmissionView } from "@/components/SubmissionView";
import { parseIdSegment } from "@/lib/routes";
import { getSubmission } from "@/lib/serverApi";
import { DEV_AUTH_AVAILABLE } from "@/lib/session";

type Params = Promise<{ id: string }>;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const id = parseIdSegment((await params).id);
  const submission = id === null ? null : await getSubmission(id);
  return { title: submission ? `提出 #${submission.id} (${submission.problem_title})` : "提出が見つかりません" };
}

export default async function SubmissionPage({ params }: { params: Params }) {
  const id = parseIdSegment((await params).id);
  if (id === null) {
    notFound();
  }
  const submission = await getSubmission(id);
  if (!submission) {
    // A private submission may belong to a dev-header actor, which the server
    // cannot see; let the browser try. Production has no dev auth.
    if (DEV_AUTH_AVAILABLE) {
      return <SubmissionView submissionId={id} />;
    }
    notFound();
  }

  return <SubmissionView submissionId={id} initial={submission} />;
}
