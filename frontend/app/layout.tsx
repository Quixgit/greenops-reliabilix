import type { ReactNode } from "react";
import { Providers } from "@/components/providers";
import { I18nProvider } from "@/i18n/provider";
import { getLocale, getLocaleConfig, getMessages } from "@/i18n/server";
import "./globals.css";

export const metadata = { title: "Reliabilix GreenOps", description: "Cloud cost and carbon footprint in one view" };

export default async function RootLayout({ children }: { children: ReactNode }) {
  const config = await getLocaleConfig();
  const locale = await getLocale(config);
  const { messages, fallback } = await getMessages(locale);
  return (
    <html lang={locale}>
      <body>
        <I18nProvider locale={locale} messages={messages} fallback={fallback} enabled={config.enabled}>
          <Providers>{children}</Providers>
        </I18nProvider>
      </body>
    </html>
  );
}
