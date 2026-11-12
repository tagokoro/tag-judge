import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { NamespaceView } from "@/components/NamespaceView";
import { getNamespace, serverClient } from "@/lib/serverApi";

type Params = Promise<{ namespace: string }>;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const namespace = await getNamespace((await params).namespace);
  return { title: namespace ? `${namespace.display_name} (@${namespace.slug})` : "Namespace が見つかりません" };
}

export default async function NamespacePage({ params }: { params: Params }) {
  const { namespace: namespaceSlug } = await params;
  const namespace = await getNamespace(namespaceSlug);
  if (!namespace) {
    notFound();
  }
  const libraries = await (await serverClient()).listLibraries(namespaceSlug);

  return <NamespaceView namespaceSlug={namespaceSlug} initial={{ namespace, libraries }} />;
}
