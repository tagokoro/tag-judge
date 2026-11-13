"use client";

import type { ReactNode } from "react";
import { OwnerOnly, useUserLibrary } from "./OwnerOnly";
import { Breadcrumbs } from "./ui";
import { libraryPath, categoryPath, userPath } from "@/lib/routes";
import type { Library } from "@/lib/types";

/**
 * The frame every library edit page shares: breadcrumbs, the heading, and the
 * owner gate. Only the owner reaches `children`.
 */
export function LibraryEditFrame({
  handle,
  libraryId,
  title,
  crumb,
  children,
}: {
  handle: string;
  libraryId: string;
  title: string;
  crumb: string;
  children: (library: Library, backHref: string) => ReactNode;
}) {
  const library = useUserLibrary(handle, libraryId);
  const backHref = libraryPath(handle, libraryId);
  return (
    <div className="page">
      {library.data ? (
        <Breadcrumbs
          items={[
            { label: `@${handle}`, href: userPath(handle) },
            { label: library.data.category.name, href: categoryPath(handle, library.data.category.id) },
            { label: library.data.name, href: backHref },
            { label: crumb },
          ]}
        />
      ) : null}
      <h1>{title}</h1>
      <OwnerOnly resource={library}>{(data) => children(data, backHref)}</OwnerOnly>
    </div>
  );
}
