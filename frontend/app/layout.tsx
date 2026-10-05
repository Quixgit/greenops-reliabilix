import type { ReactNode } from "react";
import { Providers } from "@/components/providers";
import "./globals.css";

export const metadata = { title: "Reliabilix GreenOps", description: "Cloud cost and carbon footprint in one view" };

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body><Providers>{children}</Providers></body>
    </html>
  );
}
