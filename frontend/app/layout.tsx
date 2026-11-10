import type { Metadata } from "next";
import type { ReactNode } from "react";
import { ActorScope, AuthProvider } from "@/components/AuthProvider";
import { Header } from "@/components/Header";
import "./globals.css";

// Loaded by the browser so the build never needs network access. Zen Kaku
// Gothic New is the UI face; IBM Plex Mono is used only for code, hashes and URLs.
const FONTS_URL =
  "https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500&family=Zen+Kaku+Gothic+New:wght@400;500;700&display=swap";

export const metadata: Metadata = {
  title: "TagJudge",
  description: "競技プログラミング用ライブラリを版ごとに共有する",
};

export default function RootLayout({
  children,
}: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="ja">
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="" />
        <link rel="stylesheet" href={FONTS_URL} />
      </head>
      <body>
        <AuthProvider>
          <Header />
          <main className="container">
            <ActorScope>{children}</ActorScope>
          </main>
        </AuthProvider>
      </body>
    </html>
  );
}
