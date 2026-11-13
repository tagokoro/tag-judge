import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { LibraryView } from "@/components/LibraryView";
import { descriptionSummary } from "@/lib/format";
import { getLatestVersion, getLibrary, orNull } from "@/lib/serverApi";
import { DEV_AUTH_AVAILABLE } from "@/lib/session";

type Params = Promise<{ handle: string; id: string }>;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { handle, id } = await params;
  const library = await getLibrary(id);
  if (!library || library.owner !== handle) {
    return { title: "Library が見つかりません" };
  }
  return {
    title: `${library.name} (@${handle} / ${library.category.name})`,
    description: descriptionSummary(library.description) || undefined,
  };
}

export default async function LibraryPage({ params }: { params: Params }) {
  const { handle, id } = await params;
  const [library, latest] = await Promise.all([getLibrary(id), orNull(getLatestVersion(id))]);
  if (!library || library.owner !== handle) {
    // A private library may still be visible to a dev-header actor, which the
    // server cannot see; let the browser try. Production has no dev auth.
    if (!library && DEV_AUTH_AVAILABLE) {
      return <LibraryView handle={handle} libraryId={id} />;
    }
    notFound();
  }

  return <LibraryView handle={handle} libraryId={id} initial={{ library, latest }} />;
}
