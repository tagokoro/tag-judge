"use client";

import { useAuth } from "./AuthProvider";
import { Avatar, EditLink } from "./ui";
import { SETTINGS_PROFILE_PATH } from "@/lib/routes";
import type { UserProfile } from "@/lib/types";

/** The user page body: the profile. The user's libraries come later. */
export function UserView({ profile }: { profile: UserProfile }) {
  const { actor } = useAuth();
  const owner = actor !== null && actor.handle === profile.handle;

  return (
    <div className="page">
      <section className="section profile-head">
        <Avatar name={profile.handle} large />
        <div className="profile-name">
          <h1>@{profile.handle}</h1>
        </div>
        {owner ? (
          <div className="page-actions">
            <EditLink href={SETTINGS_PROFILE_PATH} what="プロフィール" />
          </div>
        ) : null}
      </section>
    </div>
  );
}
