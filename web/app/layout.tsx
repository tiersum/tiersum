import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";

const inter = Inter({ subsets: ["latin"] });

export const metadata: Metadata = {
  title: "TierSum - Hierarchical Summary Knowledge Base",
  description: "A RAG-free document retrieval system powered by multi-layer abstraction",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="dark">
      <body className={`${inter.className} antialiased min-h-screen bg-slate-950 text-slate-50`}>
        {children}
      </body>
    </html>
  );
}
