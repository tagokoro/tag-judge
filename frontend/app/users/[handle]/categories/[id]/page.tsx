import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { CategoryView } from "@/components/CategoryView";
import { getCategory, orNull, serverClient } from "@/lib/serverApi";
import { DEV_AUTH_AVAILABLE } from "@/lib/session";

type Params = Promise<{ handle: string; id: string }>;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { handle, id } = await params;
  const category = await getCategory(id);
  return { title: category && category.owner === handle ? `${category.name} (@${handle})` : "カテゴリが見つかりません" };
}

export default async function CategoryPage({ params }: { params: Params }) {
  const { handle, id } = await params;
  const api = await serverClient();
  const [category, libraries] = await Promise.all([getCategory(id), orNull(api.listLibraries(id))]);
  if (!category || category.owner !== handle) {
    // A category without a public library may still be visible to a
    // dev-header owner, which the server cannot see; let the browser try.
    // Production has no dev auth.
    if (!category && DEV_AUTH_AVAILABLE) {
      return <CategoryView handle={handle} categoryId={id} />;
    }
    notFound();
  }
  return <CategoryView handle={handle} categoryId={id} initial={{ category, libraries: libraries ?? [] }} />;
}
