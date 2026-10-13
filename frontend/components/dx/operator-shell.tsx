"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useMemo, useRef, useState, type ReactNode } from "react";
import { Phoenix } from "@/components/layout/phoenix";
import { activeItem, ALL_ITEMS, navKey, OPERATOR_NAV } from "@/components/operator/nav";
import { useOperatorTenants } from "@/hooks/use-admin";
import { useT } from "@/i18n/provider";
import type { AppUser } from "@/lib/auth/session";
import { useUi } from "@/lib/store";
import { BUILD_VERSION } from "@/lib/version";
import { Ico } from "./icons";
import { Bell, Language } from "./shell";
import { cx, Popover } from "./ui";

/** Search for the console: its pages and the organizations. */
function ConsoleSearch() {
  const t = useT();
  const router = useRouter();
  const tenants = useOperatorTenants();
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  const all = useMemo(() => [
    ...ALL_ITEMS.map((n) => ({ label: t(navKey(n.href), undefined, n.label), hint: t("search.page"), href: n.href })),
    ...(tenants.data ?? []).map((o) => ({ label: o.name, hint: t("operator.search.organization"), href: `/operator/organizations/${o.id}` })),
  ], [t, tenants.data]);
  const items = useMemo(() => { const n = q.trim().toLowerCase(); return (n ? all.filter((i) => i.label.toLowerCase().includes(n)) : all.slice(0, 6)).slice(0, 8); }, [q, all]);
  return (
    <div style={{ position: "relative" }}>
      <label className="dx-search">
        <Ico name="search" size={18} />
        <input ref={input} value={q} placeholder={t("header.search")} aria-label={t("header.search")} onChange={(e) => { setQ(e.target.value); setOpen(true); }}
          onFocus={() => setOpen(true)} onBlur={() => window.setTimeout(() => setOpen(false), 120)}
          onKeyDown={(e) => { if (e.key === "Enter" && items[0]) { router.push(items[0].href); setOpen(false); setQ(""); } if (e.key === "Escape") setOpen(false); }} />
      </label>
      {open && items.length > 0 && (
        <div className="dx-pop is-left" style={{ width: 340, minWidth: 0 }}>
          {items.map((it) => (
            <Link key={`${it.hint}-${it.href}`} href={it.href} className="dx-opt" onMouseDown={(e) => e.preventDefault()} onClick={() => { setOpen(false); setQ(""); }}>
              <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{it.label}</span>
              <span style={{ color: "var(--dx-muted)", fontSize: 12 }}>{it.hint}</span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

/** The operator console in the same frame as the workspace: navy menu in groups, a top bar, the page head from the menu item. */
export function DxOperatorShell({ operator, user, children }: { operator: { email: string; subject: string; role: string }; user: AppUser; children: ReactNode }) {
  const t = useT();
  const path = usePathname();
  const { navOpen, setNavOpen, navCollapsed, setNavCollapsed } = useUi();
  const slim = navCollapsed;
  const current = activeItem(path);
  const initials = user.name.split(/\s+/).map((w) => w[0]).slice(0, 2).join("").toUpperCase() || "U";
  const group = OPERATOR_NAV.find((g) => g.items.some((i) => i.href === current?.href));
  return (
    <>
      <div className={cx("dx-shell", slim && "is-slim")}>
        {navOpen && <button className="dx-scrim" aria-label="Close menu" onClick={() => setNavOpen(false)} />}
        <aside className={cx("dx-side", navOpen && "is-open")} aria-label="Operator">
          <Link href="/operator" className="dx-brand" title="Reliabilix">
            <span className="dx-phx"><Phoenix /></span>
            <div><b>Reliabilix</b><small>{t("nav.operator", undefined, "Operator")}</small></div>
          </Link>
          <button type="button" className="dx-fb" aria-label={slim ? "Expand menu" : "Collapse menu"} onClick={() => setNavCollapsed(!slim)}>
            <span className="dx-fb-arrows" aria-hidden><Ico name="chevronLeft" size={16} /><Ico name="chevronLeft" size={16} /></span>
            <span className="dx-fb-text">{t("dx.nav.collapse", undefined, "Collapse")}</span>
          </button>
          <nav className="dx-nav">
            {OPERATOR_NAV.map((g) => (
              <div key={g.title}>
                <div className="dx-nav-label">{t(`operator.group.${g.title.toLowerCase()}`, undefined, g.title)}</div>
                {g.items.map((i) => {
                  const on = current?.href === i.href;
                  const text = t(navKey(i.href), undefined, i.label);
                  return (
                    <Link key={i.href} href={i.href} className={cx("dx-link", on && "is-on")} aria-current={on ? "page" : undefined} title={slim ? text : undefined} onClick={() => setNavOpen(false)}>
                      <Ico name={i.icon} size={20} /><span>{text}</span>
                      {i.planned && !slim && <em className="dx-soon">{t("operator.planned")}</em>}
                    </Link>
                  );
                })}
              </div>
            ))}
          </nav>
          <div className="dx-side-foot">
            <Link href="/overview" className="dx-link" style={{ padding: 0 }} title={slim ? t("nav.back_to_app") : undefined}><Ico name="arrowRight" size={20} style={{ transform: "rotate(180deg)" }} /><span>{t("nav.back_to_app")}</span></Link>
            <div className="dx-ver"><span>v{BUILD_VERSION.number} · {BUILD_VERSION.stage}</span></div>
          </div>
        </aside>
        <div style={{ minWidth: 0 }}>
          <header className="dx-top">
            <button type="button" className="dx-iconbtn dx-burger" aria-label={t("header.menu")} onClick={() => setNavOpen(true)} style={{ display: "none" }}><Ico name="menu" size={20} /></button>
            <div className="dx-crumb"><span>{t("nav.operator", undefined, "Operator")}</span>{group && <><Ico name="chevronRight" size={14} /><span>{t(`operator.group.${group.title.toLowerCase()}`, undefined, group.title)}</span></>}
              {current && <><Ico name="chevronRight" size={14} /><b>{t(navKey(current.href), undefined, current.label)}</b></>}</div>
            <div className="dx-top-right">
              <ConsoleSearch />
              <Language />
              <Bell />
              <Popover align="right" trigger={(open, toggle) => (
                <button type="button" className={cx("dx-iconbtn", open && "is-on")} style={{ width: "auto", gap: 10, display: "flex", alignItems: "center", padding: "0 8px" }} aria-expanded={open} onClick={toggle}>
                  <span className="dx-mark" style={{ background: "var(--dx-navy)", color: "#fff", width: 26, height: 26, fontSize: 11 }}>{initials}</span>
                  <span className="dx-user-text" style={{ textAlign: "left", lineHeight: 1.15 }}><span style={{ display: "block", fontWeight: 600, color: "var(--dx-ink)" }}>{user.name}</span><span style={{ display: "block", fontSize: 11.5, color: "var(--dx-muted)", textTransform: "capitalize" }}>{operator.role}</span></span>
                </button>
              )}>
                <div style={{ minWidth: 220 }}>
                  <div className="dx-pop-title">{user.email}</div>
                  <Link href="/overview" className="dx-opt"><Ico name="arrowRight" size={18} style={{ transform: "rotate(180deg)" }} />{t("nav.back_to_app")}</Link>
                  <a href="/auth/logout" className="dx-opt"><Ico name="logout" size={18} />{t("header.sign_out")}</a>
                </div>
              </Popover>
            </div>
          </header>
          <main className="dx-main">
            {children}
          </main>
        </div>
      </div>
      <style>{`@media (max-width: 820px) { .dx-burger { display: grid !important; } }`}</style>
    </>
  );
}
