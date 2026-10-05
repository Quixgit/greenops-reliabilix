"use client";

import { Leaf } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { content } from "@/content/schema";
import { useUi } from "@/lib/store";
import { cn } from "@/lib/utils";
import { NAV } from "./nav";

export function Sidebar() {
  const path = usePathname();
  const { navOpen, setNavOpen } = useUi();
  return (
    <>
      {navOpen && <button aria-label="Close menu" className="fixed inset-0 z-30 bg-slate-900/30 lg:hidden" onClick={() => setNavOpen(false)} />}
      <aside className={cn("fixed inset-y-0 left-0 z-40 flex w-64 flex-col gap-4 p-4 pt-20 transition-transform lg:static lg:z-auto lg:translate-x-0 lg:pt-4",
        navOpen ? "translate-x-0 bg-white/90 backdrop-blur" : "-translate-x-full")}>
        <nav aria-label="Main" className="flex flex-1 flex-col gap-1">
          {NAV.map(({ href, label, icon: Icon }) => {
            const active = path === href || path.startsWith(`${href}/`);
            return (
              <Link key={href} href={href} onClick={() => setNavOpen(false)} aria-current={active ? "page" : undefined}
                className={cn("flex items-center gap-3 rounded-xl px-3.5 py-2.5 text-sm font-medium transition",
                  active ? "brand-gradient text-white shadow-md shadow-indigo-500/25" : "text-slate-600 hover:bg-white/70")}>
                <Icon className="size-[18px]" />{label}
              </Link>
            );
          })}
        </nav>
        <div className="green-gradient relative overflow-hidden rounded-2xl p-4 text-white shadow-lg shadow-emerald-900/20">
          <Leaf className="absolute -right-3 -top-3 size-20 rotate-12 text-white/15" aria-hidden />
          <p className="relative text-sm font-semibold">{content.sidebarPromo.title}</p>
          <p className="relative mt-1 text-xs text-white/85">{content.sidebarPromo.text}</p>
        </div>
        <p className="px-1 text-xs text-muted">Build {process.env.NEXT_PUBLIC_BUILD_VERSION ?? "dev"}</p>
      </aside>
    </>
  );
}
