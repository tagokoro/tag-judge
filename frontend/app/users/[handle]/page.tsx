import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { UserView } from "@/components/UserView";
import { getUser, orNull, serverClient } from "@/lib/serverApi";

type Params = Promise<{ handle: string }>;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const profile = await getUser((await params).handle);
  return { title: profile ? `@${profile.handle}` : "ユーザーが見つかりません" };
}

export default async function UserPage({ params }: { params: Params }) {
  const { handle } = await params;
  const api = await serverClient();
  const [profile, categories] = await Promise.all([getUser(handle), orNull(api.listCategories(handle))]);
  if (!profile) {
    notFound();
  }

  return <UserView initial={{ profile, categories: categories ?? [] }} />;
}
