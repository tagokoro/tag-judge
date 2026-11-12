import type { Metadata } from "next";
import Link from "next/link";
import { HOME_PATH } from "@/lib/routes";

export const metadata: Metadata = { title: "ページが見つかりません" };

/** Missing pages, private pages of someone else, and edit pages of non-owners all land here. */
export default function NotFound() {
  return (
    <div className="page">
      <h1>ページが見つかりません</h1>
      <p className="empty">
        URL が間違っているか、private のため表示できません。 <Link href={HOME_PATH}>Libraries に戻る</Link>
      </p>
    </div>
  );
}
