import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { LibraryView } from "@/components/LibraryView";
import { descriptionSummary } from "@/lib/format";
import { getLatestVersion, getLibrary, getNamespace } from "@/lib/serverApi";
import { DEV_AUTH_AVAILABLE } from "@/lib/session";

type Params = Promise<{ namespace: string; library: string }>;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { namespace, library: librarySlug } = await params;
  const library = await getLibrary(namespace, librarySlug);
  if (!library) {
    return { title: "Library が見つかりません" };
  }
  return {
    title: `${library.name} (${namespace}/${library.slug})`,
    description: descriptionSummary(library.description) || undefined,
  };
}

export default async function LibraryPage({ params }: { params: Params }) {
  const { namespace: namespaceSlug, library: librarySlug } = await params;
  const [namespace, library] = await Promise.all([
    getNamespace(namespaceSlug),
    getLibrary(namespaceSlug, librarySlug),
  ]);
  if (!namespace || !library) {
    // A private library may still be visible to a dev-header actor, which the
    // server cannot see; let the browser try. Production has no dev auth.
    if (DEV_AUTH_AVAILABLE) {
      return <LibraryView namespaceSlug={namespaceSlug} librarySlug={librarySlug} />;
    }
    notFound();
  }
  const latest = await getLatestVersion(namespaceSlug, librarySlug);

  return (
    <LibraryView
      namespaceSlug={namespaceSlug}
      librarySlug={librarySlug}
      initial={{ namespace, library, latest }}
    />
  );
}
