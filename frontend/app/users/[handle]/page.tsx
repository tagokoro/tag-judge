import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { UserView } from "@/components/UserView";
import { getUser } from "@/lib/serverApi";

type Params = Promise<{ handle: string }>;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const profile = await getUser((await params).handle);
  return { title: profile ? `@${profile.handle}` : "ユーザーが見つかりません" };
}

export default async function UserPage({ params }: { params: Params }) {
  const profile = await getUser((await params).handle);
  if (!profile) {
    notFound();
  }

  return <UserView profile={profile} />;
}
