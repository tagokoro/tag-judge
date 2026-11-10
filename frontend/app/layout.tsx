import type { Metadata } from "next";
import type { ReactNode } from "react";
import { ActorScope, AuthProvider } from "@/components/AuthProvider";
import { Header } from "@/components/Header";
import "./globals.css";

export const metadata: Metadata = {
  title: "Library Sharing Judge",
  description: "競技プログラミング用ライブラリの共有",
};

export default function RootLayout({
  children,
}: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="ja">
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
