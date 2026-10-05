import type { ReactNode } from "react";

export const metadata = { title: "Reliabilix GreenOps" };

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
