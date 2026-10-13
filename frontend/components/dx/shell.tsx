"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { usePlanState } from "@/components/layout/plan-card";
import { eventArea, useInbox, useMarkInboxRead } from "@/hooks/use-inbox";
import { useAlerts, useConnections, useProjects } from "@/hooks/use-overview";
import { LOCALE_NAMES, isLocale } from "@/i18n/config";
import { useEnabledLocales, useLocale, useT } from "@/i18n/provider";
import type { Role } from "@/lib/permissions";
import { useUi } from "@/lib/store";
import { timeAgo } from "@/lib/formatters";
import { BUILD_VERSION } from "@/lib/version";
import { settingsGroups, SETTINGS_ALIAS } from "@/components/settings/sections";
import { Ico, type DxIconName } from "./icons";
import { cx, Popover } from "./ui";

const MAIN: { href: string; label: string; icon: DxIconName }[] = [
  { href: "/overview", label: "Overview", icon: "overview" },
  { href: "/projects", label: "Projects", icon: "projects" },
  { href: "/kubernetes", label: "Kubernetes", icon: "kubernetes" },
  { href: "/carbon", label: "Carbon & Energy", icon: "carbon" },
  { href: "/reports", label: "Reports", icon: "reports" },
  { href: "/recommendations", label: "Recommendations", icon: "recommendations" },
  { href: "/automation", label: "Automation", icon: "automation" },
  { href: "/ci-cd", label: "CI/CD", icon: "cicd" },
  { href: "/connectors", label: "Connectors", icon: "connectors" },
];
const SECONDARY: typeof MAIN = [
  { href: "/alerts", label: "Alerts", icon: "alerts" },
  { href: "/settings", label: "Settings", icon: "settings" },
];
const slug = (href: string) => href.slice(1).replace(/\W+/g, "_");

function Sidebar({ slim, isOperator }: { slim: boolean; isOperator: boolean }) {
  const t = useT();
  const path = usePathname();
  const { navOpen, setNavOpen, setNavCollapsed } = useUi();
  const plan = usePlanState();
  const link = (n: (typeof MAIN)[number]) => {
    const on = path === n.href || path.startsWith(`${n.href}/`);
    const text = t(`nav.${slug(n.href)}`, undefined, n.label);
    return (
      <Link key={n.href} href={n.href} className={cx("dx-link", on && "is-on")} aria-current={on ? "page" : undefined} title={slim ? text : undefined} onClick={() => setNavOpen(false)}>
        <Ico name={n.icon} size={20} /><span>{text}</span>
      </Link>
    );
  };
  const pct = plan?.top ? Math.min(100, (plan.top.used / plan.top.limit) * 100) : 0;
  return (
    <>
      {navOpen && <button className="dx-scrim" aria-label="Close menu" onClick={() => setNavOpen(false)} />}
      <aside className={cx("dx-side", navOpen && "is-open")} aria-label="Main">
        <Link href="/overview" className="dx-brand" title="Reliabilix">
          <span className="dx-mark">R</span>
          <div><b>Reliabilix</b><small>GreenOps</small></div>
        </Link>
        <button type="button" className="dx-fb" aria-label={slim ? "Expand menu" : "Collapse menu"} aria-expanded={!slim} title={slim ? "Expand menu" : "Collapse menu"} onClick={() => setNavCollapsed(!slim)}>
          <span className="dx-fb-arrows" aria-hidden><Ico name="chevronLeft" size={16} /><Ico name="chevronLeft" size={16} /></span>
          <span className="dx-fb-text">{t("dx.nav.collapse", undefined, "Collapse")}</span>
        </button>
        <nav className="dx-nav">
          <div className="dx-nav-label">{t("dx.nav.workspace", undefined, "Workspace")}</div>
          {MAIN.map(link)}
        </nav>
        <div className="dx-side-foot">
          {plan && (
            <Link href="/settings#plan" className="dx-plan" title={`${t("plan.card.title")}: ${plan.p.name}`}>
              <b>{t("plan.card.title")}</b>
              <strong>{plan.p.name}</strong>
              <em className="dx-plan-mini">{plan.p.name.slice(0, 2)}</em>
              {plan.top && <div className="dx-meter"><i style={{ width: `${Math.max(4, pct)}%` }} /></div>}
            </Link>
          )}
          {SECONDARY.map(link)}
          {isOperator && <Link href="/operator" className="dx-link" style={{ padding: 0 }}><Ico name="shield" size={20} /><span>{t("nav.operator", undefined, "Operator")}</span></Link>}
          <div className="dx-ver"><span>v{BUILD_VERSION.number} · {BUILD_VERSION.stage}</span></div>
        </div>
      </aside>
    </>
  );
}

/** The second menu: it slides out beside the main one while Settings is open, and its sections are grouped. */
function SettingsMenu({ role }: { role: Role }) {
  const t = useT();
  const groups = useMemo(() => settingsGroups(role, t), [role, t]);
  const [active, setActive] = useState("organization");
  useEffect(() => {
    const read = () => { const h = window.location.hash.slice(1); setActive(SETTINGS_ALIAS[h] ?? (h || "organization")); };
    read();
    window.addEventListener("hashchange", read);
    return () => window.removeEventListener("hashchange", read);
  }, []);
  return (
    <aside className="dx-sub" aria-label={t("settings.title", undefined, "Settings")}>
      <div className="dx-sub-head"><Ico name="settings" size={18} /><b>{t("settings.title", undefined, "Settings")}</b></div>
      <nav>
        {groups.map((g) => (
          <div key={g.id} className="dx-sub-group">
            <div className="dx-sub-label">{g.label}</div>
            {g.items.map((i) => (
              <a key={i.id} href={`/settings#${i.id}`} className={cx("dx-sub-link", active === i.id && "is-on")} aria-current={active === i.id ? "page" : undefined}>
                <Ico name={i.icon} size={18} />
                <span><b>{i.label}</b><small>{i.hint}</small></span>
              </a>
            ))}
          </div>
        ))}
      </nav>
    </aside>
  );
}

function Search() {
  const t = useT();
  const router = useRouter();
  const projects = useProjects();
  const conns = useConnections();
  const setProject = useUi((s) => s.setProject);
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const [cur, setCur] = useState(0);
  const input = useRef<HTMLInputElement>(null);
  const all = useMemo(() => [
    ...[...MAIN, ...SECONDARY].map((n) => ({ label: t(`nav.${slug(n.href)}`, undefined, n.label), hint: t("search.page"), run: () => router.push(n.href) })),
    ...(projects.data ?? []).map((p) => ({ label: p.name, hint: t("search.project"), run: () => { setProject(p.id); router.push("/overview"); } })),
    ...(conns.data ?? []).map((c) => ({ label: `${c.provider.toUpperCase()} ${c.account_ref}`, hint: t("search.connection"), run: () => router.push(`/cloud/${c.id}`) })),
  ], [t, router, projects.data, conns.data, setProject]);
  const items = useMemo(() => { const n = q.trim().toLowerCase(); return (n ? all.filter((i) => i.label.toLowerCase().includes(n)) : all.slice(0, 6)).slice(0, 8); }, [q, all]);
  useEffect(() => {
    const k = (e: KeyboardEvent) => { if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); input.current?.focus(); setOpen(true); } };
    window.addEventListener("keydown", k);
    return () => window.removeEventListener("keydown", k);
  }, []);
  const go = (i: number) => { items[i]?.run(); setOpen(false); setQ(""); input.current?.blur(); };
  return (
    <div style={{ position: "relative" }}>
      <label className="dx-search">
        <Ico name="search" size={18} />
        <input ref={input} value={q} placeholder={t("header.search")} aria-label={t("header.search")} role="combobox" aria-expanded={open} aria-controls="dx-search-list"
          onChange={(e) => { setQ(e.target.value); setCur(0); setOpen(true); }} onFocus={() => setOpen(true)} onBlur={() => window.setTimeout(() => setOpen(false), 120)}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown") { e.preventDefault(); setCur((c) => Math.min(items.length - 1, c + 1)); }
            else if (e.key === "ArrowUp") { e.preventDefault(); setCur((c) => Math.max(0, c - 1)); }
            else if (e.key === "Enter") { e.preventDefault(); go(cur); }
            else if (e.key === "Escape") { setOpen(false); input.current?.blur(); }
          }} />
        <span className="dx-kbd">⌘K</span>
      </label>
      {open && items.length > 0 && (
        <div id="dx-search-list" className="dx-pop is-left" style={{ width: 340, minWidth: 0 }} role="listbox">
          {items.map((it, i) => (
            <button key={`${it.hint}-${it.label}`} type="button" role="option" aria-selected={i === cur} className="dx-opt" onMouseDown={(e) => e.preventDefault()} onClick={() => go(i)} onMouseEnter={() => setCur(i)}>
              <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{it.label}</span>
              <span style={{ color: "var(--dx-muted)", fontSize: 12 }}>{it.hint}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function Bell() {
  const t = useT();
  const inbox = useInbox();
  const alerts = useAlerts("new");
  const read = useMarkInboxRead();
  const rows = [
    ...(inbox.data?.items ?? []).map((i) => ({ key: i.id, title: i.title, body: i.body, at: i.created_at, href: i.path || "/alerts", unread: i.unread, area: eventArea(i.type) })),
    ...(alerts.data ?? []).map((a) => ({ key: `a-${a.id}`, title: a.title, body: a.detail, at: a.last_seen_at, href: "/alerts#alerts", unread: true, area: "alert" })),
  ].sort((a, b) => Date.parse(b.at) - Date.parse(a.at)).slice(0, 8);
  const unread = rows.filter((r) => r.unread).length;
  return (
    <Popover align="right" trigger={(open, toggle) => (
      <button type="button" className={cx("dx-iconbtn", open && "is-on")} aria-label={t("header.notifications", undefined, "Notifications")} aria-expanded={open} onClick={() => { toggle(); if (!open && (inbox.data?.unread ?? 0) > 0) read.mutate(); }}>
        <Ico name="alerts" size={20} />{unread > 0 && <span className="dx-dot" />}
      </button>
    )}>
      <div style={{ width: 360 }}>
        <div className="dx-pop-title">{t("header.notifications", undefined, "Notifications")}</div>
        {rows.length === 0 ? <div className="dx-pop-body"><p style={{ color: "var(--dx-muted)" }}>{t("dx.bell.empty", undefined, "Nothing needs you right now.")}</p></div> : rows.map((r) => (
          <Link key={r.key} href={r.href} className="dx-opt" style={{ alignItems: "flex-start", flexDirection: "column", gap: 2 }}>
            <span style={{ fontWeight: 600 }}>{r.unread && <i style={{ display: "inline-block", width: 7, height: 7, background: "var(--dx-bad)", marginRight: 8 }} />}{r.title}</span>
            <span style={{ color: "var(--dx-muted)", fontSize: 12.5 }}>{r.body ? `${r.body.slice(0, 90)} · ` : ""}{timeAgo(r.at)}</span>
          </Link>
        ))}
        <Link href="/alerts" className="dx-opt" style={{ borderTop: "1px solid var(--dx-line)", color: "var(--dx-sky-ink)", fontWeight: 600 }}>{t("dx.bell.all", undefined, "All alerts")}<Ico name="arrowRight" size={16} className="dx-chk" /></Link>
      </div>
    </Popover>
  );
}

function Language() {
  const t = useT();
  const locale = useLocale();
  const enabled = useEnabledLocales();
  const router = useRouter();
  const choose = async (v: string) => {
    if (!isLocale(v) || v === locale) return;
    await fetch("/api/locale", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ locale: v }) });
    router.refresh();
  };
  return (
    <Popover align="right" trigger={(open, toggle) => (
      <button type="button" className={cx("dx-iconbtn", open && "is-on")} style={{ width: "auto", padding: "0 8px", gap: 6, display: "flex", alignItems: "center", fontSize: 12, fontWeight: 600 }} aria-label={t("header.language")} aria-expanded={open} onClick={toggle}>
        <Ico name="globe" size={18} />{locale.toUpperCase()}
      </button>
    )}>
      {(close) => (
        <div style={{ minWidth: 180, padding: "4px 0" }}>
          {enabled.map((l) => <button key={l} type="button" className="dx-opt" aria-current={l === locale ? "true" : undefined} onClick={() => { void choose(l); close(); }}><span>{LOCALE_NAMES[l]}</span>{l === locale && <Ico name="check" size={16} className="dx-chk" />}</button>)}
        </div>
      )}
    </Popover>
  );
}

export function DxShell({ user, isOperator, children }: { user: { name: string; email: string; role: Role }; isOperator: boolean; children: ReactNode }) {
  const t = useT();
  const collapsed = useUi((s) => s.navCollapsed);
  const setNavOpen = useUi((s) => s.setNavOpen);
  const path = usePathname();
  const page = [...MAIN, ...SECONDARY].find((n) => path === n.href || path.startsWith(`${n.href}/`));
  const inSettings = path === "/settings" || path.startsWith("/settings/");
  const here = page ? t(`nav.${slug(page.href)}`, undefined, page.label) : "";
  const initials = user.name.split(/\s+/).map((w) => w[0]).slice(0, 2).join("").toUpperCase() || "U";
  return (
    <>
      <div className={cx("dx-shell", collapsed && "is-slim", inSettings && "has-sub")}>
        <Sidebar slim={collapsed} isOperator={isOperator} />
        {inSettings && <SettingsMenu role={user.role} />}
        <div style={{ minWidth: 0 }}>
          <header className="dx-top">
            <button type="button" className="dx-iconbtn dx-burger" aria-label={t("header.menu")} onClick={() => setNavOpen(true)} style={{ display: "none" }}><Ico name="menu" size={20} /></button>
            <div className="dx-crumb"><span>Reliabilix</span><Ico name="chevronRight" size={14} /><b>{here}</b></div>
            <div className="dx-top-right">
              <Search />
              <Language />
              <Bell />
              <Popover align="right" trigger={(open, toggle) => (
                <button type="button" className={cx("dx-iconbtn", open && "is-on")} style={{ width: "auto", gap: 10, display: "flex", alignItems: "center", padding: "0 8px" }} aria-expanded={open} onClick={toggle}>
                  <span className="dx-mark" style={{ background: "var(--dx-navy)", color: "#fff", width: 26, height: 26, fontSize: 11 }}>{initials}</span>
                  <span className="dx-user-text" style={{ textAlign: "left", lineHeight: 1.15 }}><span style={{ display: "block", fontWeight: 600, color: "var(--dx-ink)" }}>{user.name}</span><span style={{ display: "block", fontSize: 11.5, color: "var(--dx-muted)", textTransform: "capitalize" }}>{user.role}</span></span>
                </button>
              )}>
                <div style={{ minWidth: 220 }}>
                  <div className="dx-pop-title">{user.email}</div>
                  <Link href="/settings" className="dx-opt"><Ico name="settings" size={18} />{t("header.settings")}</Link>
                  <a href="/auth/logout" className="dx-opt"><Ico name="logout" size={18} />{t("header.sign_out")}</a>
                </div>
              </Popover>
            </div>
          </header>
          <main className="dx-main">{children}</main>
        </div>
      </div>
      <style>{`@media (max-width: 820px) { .dx-burger { display: grid !important; } }`}</style>
    </>
  );
}
