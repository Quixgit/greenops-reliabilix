import type { LucideIcon } from "lucide-react";
import type { ComponentType, ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * The platform's own icon set. Drawn on a 24-unit grid with one pen (round caps, a slightly loose hand) and a soft
 * second tone behind the main shape, so a screen reads as one illustration instead of a pile of stock glyphs.
 * Every icon takes the usual `className` and `strokeWidth` and follows the text colour.
 */
export type IconProps = { className?: string; strokeWidth?: number; "aria-label"?: string };
export type IconType = ComponentType<IconProps>;
/** Either set: the screens still holding a stock glyph keep working while the drawn set takes over. */
export type AnyIcon = LucideIcon | IconType;

function make(name: string, tone: ReactNode, line: ReactNode): IconType {
  const Icon = ({ className, strokeWidth = 1.6, "aria-label": label }: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={strokeWidth} strokeLinecap="round" strokeLinejoin="round" className={cn("size-5 shrink-0", className)} aria-hidden={label ? undefined : true} aria-label={label} role={label ? "img" : undefined}>
      {tone && <g fill="currentColor" stroke="none" opacity=".15">{tone}</g>}
      {line}
    </svg>
  );
  Icon.displayName = `Icon(${name})`;
  return Icon;
}

export const IconOverview = make("overview",
  <path d="M3.6 4.4c0-.5.4-.9.9-.9h5.3c.5 0 .9.4.9.9v6.3c0 .5-.4.9-.9.9H4.5a.9.9 0 0 1-.9-.9z" />,
  <><path d="M3.6 4.5c0-.6.4-1 1-1h5c.6 0 1 .4 1 1v6.2c0 .6-.4 1-1 1h-5c-.6 0-1-.4-1-1z" /><path d="M14.3 3.5h4.9c.6 0 1.1.5 1.1 1.1v2.6c0 .6-.5 1.1-1.1 1.1h-4.9c-.6 0-1.1-.5-1.1-1.1V4.6c0-.6.5-1.1 1.1-1.1z" /><path d="M14.3 12.6h4.9c.6 0 1.1.5 1.1 1.1v5.7c0 .6-.5 1.1-1.1 1.1h-4.9c-.6 0-1.1-.5-1.1-1.1v-5.7c0-.6.5-1.1 1.1-1.1z" /><path d="M3.6 15.7c0-.6.5-1.1 1.1-1.1h4.9c.6 0 1.1.5 1.1 1.1v3.6c0 .6-.5 1.1-1.1 1.1H4.7c-.6 0-1.1-.5-1.1-1.1z" /></>);

export const IconFolder = make("folder",
  <path d="M3.4 8.2c0-1 .8-1.8 1.8-1.8h4l1.7 2h7.9c1 0 1.8.8 1.8 1.8v7.6c0 1-.8 1.8-1.8 1.8H5.2c-1 0-1.8-.8-1.8-1.8z" />,
  <><path d="M3.5 7.6c0-1.1.9-2 2-2h3.6c.5 0 1 .2 1.3.6l1.1 1.3h6.9c1.1 0 2 .9 2 2v7.7c0 1.1-.9 2-2 2H5.5c-1.1 0-2-.9-2-2z" /><path d="M3.8 10.2h16.4" /></>);

export const IconCluster = make("cluster",
  <path d="M12 3.2l7.2 4.1v8.4L12 19.8l-7.2-4.1V7.3z" />,
  <><path d="M12 2.9l7.5 4.3v8.7L12 20.2l-7.5-4.3V7.2z" /><circle cx="12" cy="11.6" r="2.3" /><path d="M12 9.3V6.2M14.1 12.7l2.7 1.6M9.9 12.7l-2.7 1.6" /></>);

export const IconLeaf = make("leaf",
  <path d="M5.2 18.9c-1.1-5.6 1.6-11.7 7.4-13.4 2.4-.7 5.1-.7 6.6-.5.4 2.2.2 5.2-.9 7.7-1.8 4-6.6 6.7-13.1 6.2z" />,
  <><path d="M5.3 19c-1.2-5.8 1.5-12 7.5-13.7 2.4-.7 5-.7 6.5-.5.4 2.1.3 5.2-.8 7.6-1.9 4.1-6.7 6.9-13.2 6.6z" /><path d="M5 20.4c2.2-4.6 5.2-8 9.6-10.8" /></>);

export const IconReport = make("report",
  <path d="M6.2 3.4h7.4l4.6 4.6v11.1c0 .9-.7 1.5-1.5 1.5H6.2c-.8 0-1.5-.7-1.5-1.5V4.9c0-.8.7-1.5 1.5-1.5z" />,
  <><path d="M6.4 3.3h7l4.8 4.9v10.9c0 1-.8 1.7-1.7 1.7H6.4c-1 0-1.7-.8-1.7-1.7V5c0-1 .8-1.7 1.7-1.7z" /><path d="M13.2 3.5v4.6h4.8" /><path d="M8.4 17v-3M11.6 17v-5.2M14.8 17v-2.2" /></>);

export const IconBulb = make("bulb",
  <path d="M12 3.3a6 6 0 0 0-3.4 10.9c.6.5.9 1.1.9 1.8h5c0-.7.3-1.3.9-1.8A6 6 0 0 0 12 3.3z" />,
  <><path d="M12 3.2a6.1 6.1 0 0 0-3.6 11c.6.5.9 1.1.9 1.8v.4h5.4V16c0-.7.3-1.3.9-1.8a6.1 6.1 0 0 0-3.6-11z" /><path d="M9.7 19.2h4.6M10.6 21.2h2.8" /><path d="M10.2 11.4c.8-1.4 2.3-1.6 3.6-.9" /></>);

export const IconFlow = make("flow",
  <circle cx="12" cy="12" r="3.2" />,
  <><circle cx="12" cy="12" r="3.1" /><path d="M5.4 9.8A7 7 0 0 1 17.2 6.1l1.4 1.5" /><path d="M18.9 4.7v3.4h-3.4" /><path d="M18.6 14.2a7 7 0 0 1-11.8 3.7l-1.4-1.5" /><path d="M5.1 19.3v-3.4h3.4" /></>);

export const IconBranch = make("branch",
  <circle cx="6.6" cy="6.2" r="2.4" />,
  <><circle cx="6.5" cy="5.9" r="2.3" /><circle cx="6.5" cy="18.1" r="2.3" /><circle cx="17.6" cy="9" r="2.3" /><path d="M6.5 8.2v7.6" /><path d="M17.6 11.3c0 3.5-2.5 4.1-5.2 4.4-2 .2-3.5.7-3.6 1.3" /></>);

export const IconPlug = make("plug",
  <path d="M7.4 9.2h9.2v3.2a4.6 4.6 0 0 1-9.2 0z" />,
  <><path d="M7.3 9h9.4v3.4a4.7 4.7 0 0 1-9.4 0z" /><path d="M9.6 9V4.4M14.4 9V4.4" /><path d="M12 17.2v3.3" /></>);

export const IconBell = make("bell",
  <path d="M6.2 16.4c1-1.1 1.4-2.4 1.4-4.5V10a4.4 4.4 0 0 1 8.8 0v1.9c0 2.1.4 3.4 1.4 4.5z" />,
  <><path d="M6 16.6c1.1-1.2 1.5-2.5 1.5-4.7V10a4.5 4.5 0 0 1 9 0v1.9c0 2.2.4 3.5 1.5 4.7z" /><path d="M5.3 16.6h13.4" /><path d="M10.2 19.3c.5.8 1.1 1.1 1.8 1.1s1.3-.3 1.8-1.1" /><path d="M12 3.2V4.6" /></>);

export const IconSliders = make("sliders",
  <><circle cx="8.4" cy="7.8" r="2.4" /><circle cx="15.8" cy="16.4" r="2.4" /></>,
  <><path d="M3.6 7.8h2.5M10.7 7.8h9.7" /><circle cx="8.4" cy="7.8" r="2.3" /><path d="M3.6 16.4h9.8M18.1 16.4h2.3" /><circle cx="15.8" cy="16.4" r="2.3" /></>);

export const IconShield = make("shield",
  <path d="M12 3.2c2.4 1.6 4.8 2.2 7.2 2.3v5.6c0 4.2-2.7 7.5-7.2 9.6-4.5-2.1-7.2-5.4-7.2-9.6V5.5c2.4-.1 4.8-.7 7.2-2.3z" />,
  <><path d="M12 2.9c2.4 1.6 4.9 2.3 7.4 2.4v5.9c0 4.3-2.8 7.7-7.4 9.8-4.6-2.1-7.4-5.5-7.4-9.8V5.3c2.5-.1 5-.8 7.4-2.4z" /><path d="M8.9 11.9l2.3 2.3 4-4.4" /></>);

export const IconCoin = make("coin",
  <circle cx="12" cy="12" r="8.4" />,
  <><path d="M20.1 12a8.1 8.1 0 1 1-2.4-5.7" /><path d="M19.5 3.9v3.2h-3.2" /><path d="M14.4 9.3c-.5-.7-1.4-1-2.4-1-1.4 0-2.4.7-2.4 1.8 0 2.4 4.9 1.1 4.9 3.5 0 1.1-1.1 1.9-2.5 1.9-1.1 0-2-.4-2.6-1.2" /><path d="M12 7v1.4M12 15.8v1.3" /></>);

export const IconCloud = make("cloud",
  <path d="M7.4 18.4a4.3 4.3 0 0 1-.6-8.5 5.6 5.6 0 0 1 10.7 1.1 3.7 3.7 0 0 1-.4 7.4z" />,
  <path d="M7.5 18.6a4.4 4.4 0 0 1-.7-8.7 5.7 5.7 0 0 1 10.9 1.2 3.8 3.8 0 0 1-.4 7.5z" />);

export const IconBolt = make("bolt",
  <path d="M13.2 3.2L5.4 13.4h5.2l-1 7.4 8-10.4h-5.3z" />,
  <path d="M13.3 3L5.3 13.5h5.3L9.6 21l8.2-10.6h-5.4z" />);

export const IconGauge = make("gauge",
  <path d="M3.9 16.5a8.4 8.4 0 1 1 16.2 0z" />,
  <><path d="M3.6 16.7a8.6 8.6 0 1 1 16.8 0" /><path d="M12 15.1l3.4-5.1" /><circle cx="12" cy="15.4" r="1.2" /><path d="M6.4 11.3l1 .6M12 7.4v1.2M17.6 11.3l-1 .6" /></>);

export const IconUsers = make("users",
  <circle cx="9.2" cy="8.6" r="3.2" />,
  <><circle cx="9.1" cy="8.5" r="3.1" /><path d="M3.4 19.4c.3-3.2 2.8-5 5.7-5s5.4 1.8 5.7 5" /><path d="M15.7 5.6a3 3 0 0 1 0 5.8" /><path d="M17.4 14.7c1.8.5 3 2 3.2 4.4" /></>);

export const IconKey = make("key",
  <circle cx="8.2" cy="12.4" r="4.2" />,
  <><circle cx="8.2" cy="12.3" r="4.1" /><path d="M12.3 12.2h8.1" /><path d="M17.4 12.2v2.7M20.2 12.2v2" /><circle cx="8.2" cy="12.3" r="1" /></>);

export const IconLock = make("lock",
  <path d="M5.8 11h12.4c.6 0 1 .4 1 1v6.6c0 .6-.4 1-1 1H5.8c-.6 0-1-.4-1-1V12c0-.6.4-1 1-1z" />,
  <><path d="M5.7 10.8h12.6c.7 0 1.2.5 1.2 1.2v6.9c0 .7-.5 1.2-1.2 1.2H5.7c-.7 0-1.2-.5-1.2-1.2V12c0-.7.5-1.2 1.2-1.2z" /><path d="M8.3 10.8V8a3.7 3.7 0 0 1 7.4 0v2.8" /><path d="M12 14.4v2.2" /></>);

export const IconGlobe = make("globe",
  <circle cx="12" cy="12" r="8.4" />,
  <><circle cx="12" cy="12" r="8.5" /><path d="M3.7 12h16.6" /><path d="M12 3.5c-2.5 2.3-3.6 5.1-3.6 8.5s1.1 6.2 3.6 8.5c2.5-2.3 3.6-5.1 3.6-8.5S14.5 5.8 12 3.5z" /></>);

export const IconChart = make("chart",
  <path d="M5 13.4h3.4v6H5zM10.3 8.6h3.4v10.8h-3.4zM15.6 11.2H19v8.2h-3.4z" />,
  <><path d="M4.4 20.2h15.4" /><path d="M5.6 19.6v-5.7h2.6v5.7M10.7 19.6V8.9h2.6v10.7M15.8 19.6v-8.1h2.6v8.1" /></>);

export const IconTrend = make("trend", null,
  <><path d="M3.6 17.6l5.2-5.4 3.6 3.2 7.6-8" /><path d="M15.2 7.3h4.8v4.7" /></>);

export const IconWallet = make("wallet",
  <path d="M4.2 7.8c0-1 .8-1.8 1.8-1.8h11.4c.9 0 1.6.7 1.6 1.6V18c0 1-.8 1.8-1.8 1.8H6c-1 0-1.8-.8-1.8-1.8z" />,
  <><path d="M4.3 7.9c0-1.1.9-2 2-2h10.7c1 0 1.8.8 1.8 1.8V18c0 1.1-.9 2-2 2H6.3c-1.1 0-2-.9-2-2z" /><path d="M4.4 8.4c.4.5 1 .8 1.7.8h13.7" /><circle cx="16.1" cy="14" r="1.1" /></>);

export const IconMail = make("mail",
  <path d="M4.4 6.8c0-.7.6-1.3 1.3-1.3h12.6c.7 0 1.3.6 1.3 1.3v10.4c0 .7-.6 1.3-1.3 1.3H5.7c-.7 0-1.3-.6-1.3-1.3z" />,
  <><path d="M4.5 6.9c0-.8.7-1.5 1.5-1.5h12c.8 0 1.5.7 1.5 1.5v10.2c0 .8-.7 1.5-1.5 1.5H6c-.8 0-1.5-.7-1.5-1.5z" /><path d="M4.9 7.6l7.1 5.2 7.1-5.2" /></>);

export const IconClock = make("clock",
  <circle cx="12" cy="12" r="8.4" />,
  <><circle cx="12" cy="12" r="8.5" /><path d="M12 7.2V12l3.2 2" /></>);

export const IconDatabase = make("database",
  <path d="M5 6.6c0-1.5 3.1-2.6 7-2.6s7 1.1 7 2.6v10.8c0 1.5-3.1 2.6-7 2.6s-7-1.1-7-2.6z" />,
  <><ellipse cx="12" cy="6.4" rx="7" ry="2.8" /><path d="M5 6.6v10.7c0 1.6 3.1 2.8 7 2.8s7-1.2 7-2.8V6.6" /><path d="M5 12c0 1.6 3.1 2.8 7 2.8s7-1.2 7-2.8" /></>);

export const IconTarget = make("target",
  <circle cx="12" cy="12" r="8.4" />,
  <><circle cx="12" cy="12" r="8.5" /><circle cx="12" cy="12" r="4.6" /><circle cx="12" cy="12" r="1.1" /></>);

export const IconPin = make("pin",
  <path d="M12 3.4a5.8 5.8 0 0 0-5.8 5.8c0 4.4 5.8 11 5.8 11s5.8-6.6 5.8-11A5.8 5.8 0 0 0 12 3.4z" />,
  <><path d="M12 3.2a6 6 0 0 0-6 6c0 4.6 6 11.3 6 11.3s6-6.7 6-11.3a6 6 0 0 0-6-6z" /><circle cx="12" cy="9.3" r="2.2" /></>);

export const IconWarning = make("warning",
  <path d="M12 4.3l8.2 14.3H3.8z" />,
  <><path d="M11.9 4.1c.1-.1.2-.1.3 0l8.3 14.4c.1.2 0 .5-.3.5H3.7c-.3 0-.4-.3-.3-.5z" /><path d="M12 9.8v4.2M12 16.6v.1" /></>);

export const IconCheck = make("check",
  <circle cx="12" cy="12" r="8.4" />,
  <><circle cx="12" cy="12" r="8.5" /><path d="M8.1 12.3l2.7 2.7 5.2-5.6" /></>);

export const IconClipboard = make("clipboard",
  <path d="M6.2 5.6h11.6c.6 0 1.1.5 1.1 1.1v12.2c0 .6-.5 1.1-1.1 1.1H6.2c-.6 0-1.1-.5-1.1-1.1V6.7c0-.6.5-1.1 1.1-1.1z" />,
  <><path d="M6.2 5.4h11.6c.7 0 1.3.6 1.3 1.3v12.2c0 .7-.6 1.3-1.3 1.3H6.2c-.7 0-1.3-.6-1.3-1.3V6.7c0-.7.6-1.3 1.3-1.3z" /><path d="M9 5.4V4.2c0-.4.3-.7.7-.7h4.6c.4 0 .7.3.7.7v1.2" /><path d="M8.4 11h7.2M8.4 14.4h5" /></>);

export const IconBuilding = make("building",
  <path d="M5.6 20.2V6.4c0-.6.4-1 1-1h6.5c.6 0 1 .4 1 1v13.8z" />,
  <><path d="M5.5 20.4V6.3c0-.7.5-1.2 1.2-1.2h6.4c.7 0 1.2.5 1.2 1.2v14.1" /><path d="M14.3 10.2h3.3c.7 0 1.2.5 1.2 1.2v9" /><path d="M3.9 20.4h16.4" /><path d="M8.6 8.6h2.2M8.6 11.8h2.2M8.6 15h2.2" /></>);

export const IconSend = make("send",
  <path d="M20.2 4L9.4 14.2l-5.3-2.1z" />,
  <><path d="M20.4 3.8L3.9 11.3l5.5 2.2 2.2 5.5z" /><path d="M9.4 13.5L20.4 3.8" /></>);

export const IconPlay = make("play",
  <path d="M8 5.6v12.8l10.6-6.4z" />,
  <path d="M7.8 5.2c0-.4.4-.6.7-.4l10.3 6.8c.3.2.3.6 0 .8L8.5 19.2c-.3.2-.7 0-.7-.4z" />);

export const IconRefresh = make("refresh", null,
  <><path d="M19.6 12a7.6 7.6 0 1 1-2.5-5.6" /><path d="M19.8 4.2v4h-4" /></>);

export const IconPlus = make("plus", null, <path d="M12 5v14M5 12h14" />);
export const IconArrowRight = make("arrow-right", null, <path d="M4.5 12h15M13.5 6l6 6-6 6" />);
export const IconChevronRight = make("chevron-right", null, <path d="M9.5 5.5l6.5 6.5-6.5 6.5" />);
export const IconInfo = make("info", <circle cx="12" cy="12" r="8.6" />, <><circle cx="12" cy="12" r="8.5" /><path d="M12 11v5.2" /><path d="M12 7.7v.1" /></>);

export const IconClose = make("close", null, <><path d="M6.2 6.4c3.8 3.6 7.6 7.4 11.6 11.4" /><path d="M17.7 6.2C13.9 9.9 10.2 13.8 6.3 17.7" /></>);

export const IconSearch = make("search", <circle cx="10.6" cy="10.6" r="6.4" />, <><circle cx="10.6" cy="10.6" r="6.3" /><path d="M15.4 15.5l4.6 4.5" /></>);

// ---- The four platforms, drawn with the same pen as the rest, bare (no tile, no frame) -----------------------------
/* Platform marks: our own drawings (not the vendors' logos), each in its own soft colours so they read at a glance. */
export function mark(name: string, body: ReactNode): IconType {
  const Icon = ({ className }: IconProps) => (
    <svg viewBox="0 0 32 32" fill="none" strokeLinecap="round" strokeLinejoin="round" className={cn("size-10 shrink-0", className)} aria-hidden>{body}</svg>
  );
  Icon.displayName = `Icon(${name})`;
  return Icon;
}

export const IconAws = mark("aws",
  <>
    <path d="M16 3.6l10.6 5.2v10.4L16 24.4 5.4 19.2V8.8z" fill="#fbe3b0" />
    <path d="M16 3.6l10.6 5.2L16 14 5.4 8.8z" fill="#f6c768" />
    <path d="M16 14v10.4l10.6-5.2V8.8z" fill="#eaa93f" />
    <path d="M16 14v10.4L5.4 19.2V8.8z" fill="#f4bd5a" />
    <path d="M7.4 26.2c5.6 2.6 12.4 2.4 17.6-.6" stroke="#c9852a" strokeWidth="1.8" />
    <path d="M22.6 25.2l2.6.4-.6 2.6" stroke="#c9852a" strokeWidth="1.8" />
  </>);

export const IconAzure = mark("azure",
  <>
    <path d="M13.2 4.6 3.6 26.4h7.2L21.4 6.2z" fill="#9fc3f5" />
    <path d="M17 11.4 28.4 27.4H11.2l5.4-3.6 4 .4z" fill="#5b8fdc" />
    <path d="M13.2 4.6l8.2 1.6L11 26.4H3.6z" fill="#7aa7ea" opacity=".85" />
  </>);

export const IconGcp = mark("gcp",
  <>
    <path d="M9.4 24.8a6.4 6.4 0 0 1-1.2-12.6 8.4 8.4 0 0 1 16 1.6 5.5 5.5 0 0 1-1.4 11z" fill="#e6f0fd" stroke="#8db4ee" strokeWidth="1.6" />
    <path d="M16 12.4l4.2 2.4v4.8L16 22l-4.2-2.4v-4.8z" fill="#fff" stroke="#8db4ee" strokeWidth="1.4" />
    <path d="M16 12.4l4.2 2.4L16 17.2z" fill="#eba7b3" />
    <path d="M20.2 14.8v4.8L16 17.2z" fill="#f2cf8a" />
    <path d="M16 22l-4.2-2.4L16 17.2z" fill="#8bd0b4" />
    <path d="M11.8 14.8 16 17.2l-4.2 2.4z" fill="#8db4ee" />
  </>);

export const IconKubernetes = mark("kubernetes",
  <>
    <path d="M16 3.4l10.2 4.9 2.5 11-7 8.8H10.3l-7-8.8 2.5-11z" fill="#c9d7f7" />
    <path d="M16 3.4l10.2 4.9 2.5 11-7 8.8H10.3l-7-8.8 2.5-11z" stroke="#6f8fe0" strokeWidth="1.6" />
    <circle cx="16" cy="16.4" r="3.2" fill="#fff" stroke="#6f8fe0" strokeWidth="1.6" />
    <path d="M16 13.2V8.6M18.8 14.8l4-2.2M18.6 18.2l3.6 3M13.4 18.2l-3.6 3M13.2 14.8l-4-2.2" stroke="#6f8fe0" strokeWidth="1.8" />
  </>);

/* Second batch: the glyphs the operator console and the rarer forms still took from the stock set. */
export const IconServer = make("server",
  <path d="M4.4 4.6c0-.6.5-1.1 1.1-1.1h13c.6 0 1.1.5 1.1 1.1v3.6c0 .6-.5 1.1-1.1 1.1h-13c-.6 0-1.1-.5-1.1-1.1z" />,
  <><path d="M4.5 4.7c0-.7.5-1.2 1.2-1.2h12.6c.7 0 1.2.5 1.2 1.2v3.5c0 .7-.5 1.2-1.2 1.2H5.7c-.7 0-1.2-.5-1.2-1.2z" /><path d="M4.5 15.8c0-.7.5-1.2 1.2-1.2h12.6c.7 0 1.2.5 1.2 1.2v3.5c0 .7-.5 1.2-1.2 1.2H5.7c-.7 0-1.2-.5-1.2-1.2z" /><path d="M7.8 6.5h.01M7.8 17.5h.01" /><path d="M12 9.4v5.2" /></>);

export const IconNetwork = make("network",
  <circle cx="12" cy="6" r="2.6" />,
  <><circle cx="12" cy="5.9" r="2.5" /><circle cx="5.9" cy="18" r="2.5" /><circle cx="18.1" cy="18" r="2.5" /><path d="M10.9 8.2 7 15.7M13.1 8.2l3.9 7.5M8.4 18h7.2" /></>);

export const IconShare = make("share",
  <circle cx="17.5" cy="6.2" r="2.6" />,
  <><circle cx="17.5" cy="6" r="2.5" /><circle cx="6.3" cy="12" r="2.5" /><circle cx="17.5" cy="18" r="2.5" /><path d="M8.5 10.8l7-3.6M8.5 13.2l7 3.6" /></>);

export const IconPalette = make("palette",
  <path d="M12 3.6a8.4 8.4 0 1 0 0 16.8c1.2 0 1.7-.9 1.4-1.8-.4-1 .3-2.1 1.4-2.1h2.2a2.6 2.6 0 0 0 2.6-2.6C19.6 7.1 16.2 3.6 12 3.6z" />,
  <><path d="M12 3.4a8.6 8.6 0 1 0 0 17.2c1.3 0 1.8-1 1.4-1.9-.4-1 .3-2.2 1.5-2.2h2.1a2.7 2.7 0 0 0 2.7-2.7c0-5.1-3.7-10.4-7.7-10.4z" /><path d="M7.6 11.2h.01M10 7.6h.01M14.4 7.8h.01" /></>);

export const IconEye = make("eye",
  <ellipse cx="12" cy="12" rx="8.6" ry="5.4" />,
  <><path d="M2.9 12c1.9-3.6 5.1-5.7 9.1-5.7s7.2 2.1 9.1 5.7c-1.9 3.6-5.1 5.7-9.1 5.7S4.8 15.6 2.9 12z" /><circle cx="12" cy="12" r="2.8" /></>);

export const IconEyeOff = make("eye-off",
  <ellipse cx="12" cy="12" rx="8.6" ry="5.4" />,
  <><path d="M9.9 6.6a9.6 9.6 0 0 1 2.1-.3c4 0 7.2 2.1 9.1 5.7-.7 1.3-1.5 2.4-2.5 3.3M6.2 8.3A13 13 0 0 0 2.9 12c1.9 3.6 5.1 5.7 9.1 5.7 1.3 0 2.5-.2 3.6-.7" /><path d="M10 10.2a2.8 2.8 0 0 0 3.7 3.7" /><path d="M4 4l16 16" /></>);

export const IconCable = make("cable",
  <path d="M8 3.6h8v5H8z" />,
  <><path d="M8.2 3.5h7.6v5.2H8.2z" /><path d="M10.2 3.5V2M13.8 3.5V2" /><path d="M12 8.7v4.8c0 1.8-1.4 3-3 3H8" /><path d="M6 14.5v4M3.5 18.5h5" /></>);

export const IconChevronDown = make("chevron-down", null, <path d="M6 9.5l6 5.6 6-5.6" />);
export const IconChevronLeft = make("chevron-left", null, <path d="M14.5 6l-5.6 6 5.6 6" />);
export const IconChevronsUpDown = make("chevrons-up-down", null, <path d="M7.5 9.2 12 4.8l4.5 4.4M7.5 14.8 12 19.2l4.5-4.4" />);
export const IconArrowUp = make("arrow-up", null, <path d="M12 19.2V5M6 10.8 12 5l6 5.8" />);
export const IconArrowDown = make("arrow-down", null, <path d="M12 4.8V19M6 13.2 12 19l6-5.8" />);
export const IconArrowLeft = make("arrow-left", null, <path d="M19.2 12H5M10.8 6 5 12l5.8 6" />);
export const IconArrowUpRight = make("arrow-up-right", null, <path d="M6.5 17.5 17 7M9 6.6h8.4V15" />);
export const IconMinus = make("minus", null, <path d="M5.5 12h13" />);

export const IconPencil = make("pencil",
  <path d="M15.4 5.2l3.4 3.4-9.6 9.6-4.2.8.8-4.2z" />,
  <><path d="M15.2 4.9a2 2 0 0 1 2.9 0l1 1a2 2 0 0 1 0 2.9L9.3 18.7l-4.6 1 1-4.6z" /><path d="M13.6 6.6l3.8 3.8" /></>);

export const IconTrash = make("trash",
  <path d="M6.4 7.6h11.2l-.9 11.2c-.1.9-.8 1.6-1.7 1.6H9c-.9 0-1.6-.7-1.7-1.6z" />,
  <><path d="M4.4 7.2h15.2" /><path d="M9.4 7V4.9c0-.5.4-.9.9-.9h3.4c.5 0 .9.4.9.9V7" /><path d="M6.5 7.2l.9 11.6c.1.9.8 1.6 1.7 1.6h5.8c.9 0 1.6-.7 1.7-1.6l.9-11.6" /><path d="M10 11v5.4M14 11v5.4" /></>);

export const IconCard = make("card",
  <path d="M3.6 6.8c0-.8.6-1.4 1.4-1.4h14c.8 0 1.4.6 1.4 1.4v2.4H3.6z" />,
  <><path d="M3.6 7c0-1 .8-1.8 1.8-1.8h13.2c1 0 1.8.8 1.8 1.8v10c0 1-.8 1.8-1.8 1.8H5.4c-1 0-1.8-.8-1.8-1.8z" /><path d="M3.7 9.8h16.6" /><path d="M7 15h3.2" /></>);

export const IconRepeat = make("repeat",
  null,
  <><path d="M5 11.2V10c0-1.7 1.3-3 3-3h10.2M15.6 4.2 18.4 7l-2.8 2.8" /><path d="M19 12.8V14c0 1.7-1.3 3-3 3H5.8M8.4 19.8 5.6 17l2.8-2.8" /></>);

export const IconHistory = make("history",
  <circle cx="12.6" cy="12" r="7.4" />,
  <><path d="M4.2 12a8 8 0 1 0 2.5-5.8L4.2 8.6" /><path d="M4.1 4.6v4h4" /><path d="M12.4 7.8V12l3 1.8" /></>);

export const IconUndo = make("undo", null,
  <><path d="M4.6 5.2v4.6h4.6" /><path d="M5 9.6A8 8 0 1 1 4.4 13" /></>);

export const IconPower = make("power",
  <circle cx="12" cy="13" r="7.4" />,
  <><path d="M12 3.6v8" /><path d="M7.2 7.2a7.4 7.4 0 1 0 9.6 0" /></>);

export const IconUserPlus = make("user-plus",
  <circle cx="9.6" cy="8.4" r="3.6" />,
  <><circle cx="9.5" cy="8.3" r="3.5" /><path d="M3.4 19.6c.5-3.3 3-5.2 6.1-5.2 1.7 0 3.2.5 4.3 1.5" /><path d="M18 9.4v5.6M15.2 12.2h5.6" /></>);

export const IconUser = make("user",
  <circle cx="12" cy="8.4" r="3.9" />,
  <><circle cx="12" cy="8.3" r="3.7" /><path d="M4.6 20c.6-3.6 3.5-5.6 7.4-5.6s6.8 2 7.4 5.6" /></>);

export const IconDownload = make("download", null,
  <><path d="M12 4v11M7.2 10.6 12 15.4l4.8-4.8" /><path d="M4.6 17.6v.9c0 .9.7 1.6 1.6 1.6h11.6c.9 0 1.6-.7 1.6-1.6v-.9" /></>);

export const IconCopy = make("copy",
  <rect x="9" y="9" width="11" height="11" rx="2.4" />,
  <><rect x="8.8" y="8.8" width="11" height="11" rx="2.4" /><path d="M15.2 8.6V6.4c0-1.2-.9-2.2-2.2-2.2H6.4c-1.2 0-2.2.9-2.2 2.2V13c0 1.2.9 2.2 2.2 2.2h2.2" /></>);

export const IconEnter = make("enter", null,
  <><path d="M19.4 5v7.4c0 1-.8 1.8-1.8 1.8H5" /><path d="M9.4 9.8 5 14.2l4.4 4.4" /></>);

export const IconCheckCircle = make("check-circle",
  <circle cx="12" cy="12" r="8.6" />,
  <><circle cx="12" cy="12" r="8.4" /><path d="M8.2 12.4l2.6 2.6 5-5.6" /></>);

export const IconCircleDashed = make("circle-dashed", null,
  <><path d="M12 3.5a8.5 8.5 0 0 1 4.4 1.2M19.3 7.6a8.5 8.5 0 0 1 1.2 4.4M20.5 12.5a8.5 8.5 0 0 1-1.2 3.9M16.4 19.3a8.5 8.5 0 0 1-4.4 1.2M7.6 19.3a8.5 8.5 0 0 1-3.2-3.2M3.5 12a8.5 8.5 0 0 1 1.2-4.4M7.6 4.7" /></>);

export const IconHourglass = make("hourglass",
  <path d="M7.4 4h9.2c0 4-2.2 5.6-4.6 8-2.4-2.4-4.6-4-4.6-8z" />,
  <><path d="M6.4 3.6h11.2M6.4 20.4h11.2" /><path d="M7.6 3.6c0 4.2 2.1 5.7 4.4 8.4-2.3 2.7-4.4 4.2-4.4 8.4M16.4 3.6c0 4.2-2.1 5.7-4.4 8.4 2.3 2.7 4.4 4.2 4.4 8.4" /></>);

export const IconCpu = make("cpu",
  <rect x="7" y="7" width="10" height="10" rx="2" />,
  <><rect x="6.8" y="6.8" width="10.4" height="10.4" rx="2.2" /><rect x="9.8" y="9.8" width="4.4" height="4.4" rx="1" /><path d="M9.5 3.6v3.2M14.5 3.6v3.2M9.5 17.2v3.2M14.5 17.2v3.2M3.6 9.5h3.2M3.6 14.5h3.2M17.2 9.5h3.2M17.2 14.5h3.2" /></>);

export const IconLogOut = make("log-out", null,
  <><path d="M9.6 4.4H6.2c-1 0-1.8.8-1.8 1.8v11.6c0 1 .8 1.8 1.8 1.8h3.4" /><path d="M15.4 8 19.6 12l-4.2 4M19.4 12H9.6" /></>);

export const IconMenu = make("menu", null, <path d="M4.6 7h14.8M4.6 12h14.8M4.6 17h14.8" />);

export const IconLanguages = make("languages",
  <circle cx="12" cy="12" r="8.6" />,
  <><path d="M4 6.6h8M8 4.4v2.2M5.6 6.6c.5 2.6 2.4 4.8 5 5.8M10.6 6.6C10 9.6 7.6 12 4.8 13" /><path d="M12.6 19.6l3.6-8.4 3.6 8.4M13.8 17h4.8" /></>);

export const IconCalculator = make("calculator",
  <rect x="5.4" y="3.4" width="13.2" height="17.2" rx="2.4" />,
  <><rect x="5.2" y="3.3" width="13.6" height="17.4" rx="2.5" /><path d="M8.4 7.6h7.2" /><path d="M8.6 12h.01M12 12h.01M15.4 12h.01M8.6 15.6h.01M12 15.6h.01M15.4 15.6h.01" /></>);

export const IconDrive = make("drive",
  <path d="M3.6 14.4h16.8v3.2c0 1-.8 1.8-1.8 1.8H5.4c-1 0-1.8-.8-1.8-1.8z" />,
  <><path d="M5.2 6.6c.3-.7 1-1.2 1.8-1.2h10c.8 0 1.5.5 1.8 1.2l1.6 7.4v3.6c0 1-.8 1.8-1.8 1.8H5.4c-1 0-1.8-.8-1.8-1.8V14z" /><path d="M3.8 14h16.4" /><path d="M16.4 16.8h.01" /></>);

export const IconTable = make("table",
  <path d="M3.6 6c0-1 .8-1.8 1.8-1.8h13.2c1 0 1.8.8 1.8 1.8v3H3.6z" />,
  <><rect x="3.5" y="4.2" width="17" height="15.6" rx="2.2" /><path d="M3.6 9.4h16.8M3.6 14.6h16.8M9.4 9.4v10.4" /></>);

export const IconLocate = make("locate",
  <circle cx="12" cy="12" r="4.4" />,
  <><circle cx="12" cy="12" r="4.2" /><path d="M12 2.8v3.4M12 17.8v3.4M2.8 12h3.4M17.8 12h3.4" /></>);

export const IconExpand = make("expand", null,
  <><path d="M14.4 4.6h5v5M9.6 19.4h-5v-5" /><path d="M19.4 4.6 13.6 10.4M4.6 19.4l5.8-5.8" /></>);

export const IconSparkles = make("sparkles",
  <path d="M10.4 5.4l1.5 4.2 4.2 1.5-4.2 1.5-1.5 4.2-1.5-4.2-4.2-1.5 4.2-1.5z" />,
  <><path d="M10.2 4.6 12 9.8l5.2 1.8-5.2 1.8-1.8 5.2-1.8-5.2-5.2-1.8 5.2-1.8z" /><path d="M18.6 3.6v3.2M17 5.2h3.2M18.4 16.8v2.8M17 18.2h2.8" /></>);

export const IconReceipt = make("receipt",
  <path d="M6.4 3.6h11.2v16.8l-2.8-1.8-2.8 1.8-2.8-1.8-2.8 1.8z" />,
  <><path d="M6.2 3.5h11.6v17.1l-2.9-1.9-2.9 1.9-2.9-1.9-2.9 1.9z" /><path d="M9.2 8.4h5.6M9.2 12.2h5.6" /></>);

export const IconTimer = make("timer",
  <circle cx="12" cy="13.4" r="7.2" />,
  <><circle cx="12" cy="13.4" r="7.1" /><path d="M9.6 3.4h4.8M12 13.4l2.8-2.8" /></>);

export const IconMerge = make("merge",
  <circle cx="6.4" cy="18" r="2.4" />,
  <><circle cx="6.4" cy="6" r="2.3" /><circle cx="6.4" cy="18" r="2.3" /><circle cx="17.6" cy="12" r="2.3" /><path d="M6.4 8.3v7.4" /><path d="M8.5 7.2c3 0 6 1.6 7 3.4M8.5 16.8c3 0 6-1.6 7-3.4" /></>);

export const IconLoader = make("loader", null,
  <path d="M12 3.6a8.4 8.4 0 1 0 8.4 8.4" />);

/* Sign-in page: one drawn picture per promise, larger, with a second tone. */
export const IconCostCarbon = make("cost-carbon",
  <><circle cx="8.6" cy="12.4" r="6" /><path d="M13.6 8.2c3.4-.2 6 1.6 6.2 5.2-3.2.7-6-.4-6.2-5.2z" /></>,
  <><circle cx="8.6" cy="12.4" r="5.8" /><path d="M8.6 9.6v5.6M10.4 10.8c-.4-.7-1.1-1-1.8-1-1 0-1.7.5-1.7 1.3 0 1.8 3.7.8 3.7 2.6 0 .8-.8 1.4-1.9 1.4-.8 0-1.6-.4-1.9-1.1" /><path d="M13.4 8.4c3.4-.4 6.2 1.4 6.5 5.1-3.3.7-6.2-.5-6.5-5.1z" /><path d="M13.6 8.7c1.6.7 3 2.2 4.2 4.4" /></>);

export const IconGridMeasure = make("grid-measure",
  <path d="M4 18.4a8.6 8.6 0 0 1 16 0z" />,
  <><path d="M3.8 18.4a8.7 8.7 0 0 1 16.4 0" /><path d="M3.8 18.4h16.4" /><path d="M12 18.4l3.6-6.4" /><circle cx="12" cy="18.4" r="1.1" /><path d="M6.4 11.2l1 .9M12 8.2v1.3M17.6 11.2l-1 .9" /><path d="M5 21h14" /></>);

export const IconInsightAction = make("insight-action",
  <path d="M3.8 15.6l5-5.2 3.4 3.4 7-7.4v9.2z" />,
  <><path d="M3.6 16.2l5.2-5.4 3.4 3.4 6.6-7" /><path d="M14.6 7.2h4.4v4.4" /><path d="M4 20h16" /><path d="M7 18.2v1.8M12 17.2v2.8M17 16.2v3.8" /></>);

export const IconTrust = make("trust",
  <path d="M12 3.4l7 2.6v5.6c0 4-2.9 7.2-7 9-4.1-1.8-7-5-7-9V6z" />,
  <><path d="M12 3.2l7.2 2.7v5.8c0 4.1-3 7.4-7.2 9.2-4.2-1.8-7.2-5.1-7.2-9.2V5.9z" /><path d="M8.6 12.2l2.5 2.6 4.4-5" /></>);

/* Grid data sources: our own marks, in the platform's soft colours, standing in for the providers' logos. */
export const IconSrcElectricityMaps = mark("src-electricity-maps",
  <>
    <path d="M16 3.6c-5 0-9 3.8-9 8.6 0 6.2 9 16.2 9 16.2s9-10 9-16.2c0-4.800-4-8.600-9-8.600z" fill="#e3f4ee" stroke="#6fc3a8" strokeWidth="1.8" />
    <path d="M17.400 8.400 12.600 15h3.400l-1.400 6.400 4.800-6.800h-3.400z" fill="#f2cf8a" stroke="#d9a441" strokeWidth="1.2" />
  </>);
export const IconSrcEnergyCharts = mark("src-energy-charts",
  <>
    <rect x="5" y="17" width="4.400" height="9" rx="1.200" fill="#8db4ee" />
    <rect x="11.800" y="10" width="4.400" height="16" rx="1.200" fill="#6fc3a8" />
    <rect x="18.600" y="5" width="4.400" height="21" rx="1.200" fill="#f2cf8a" />
    <path d="M4 27.600h24" stroke="#b9c4d6" strokeWidth="1.600" />
  </>);
export const IconSrcIeso = mark("src-ieso",
  <>
    <path d="M16 4.800c-5.200 5-8 9.400-8 13.400a8 8 0 0 0 16 0c0-4-2.800-8.400-8-13.400z" fill="#c9d7f7" stroke="#6f8fe0" strokeWidth="1.600" />
    <path d="M12.400 18.600l2.600 2.600 4.600-5.400" stroke="#6fc3a8" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" fill="none" />
  </>);
export const IconSrcHydroQuebec = mark("src-hydroquebec",
  <>
    <path d="M5 12.400c3-3 6-3 9 0s6 3 9 0" stroke="#8db4ee" strokeWidth="2.400" strokeLinecap="round" fill="none" />
    <path d="M5 18.400c3-3 6-3 9 0s6 3 9 0" stroke="#6fc3a8" strokeWidth="2.400" strokeLinecap="round" fill="none" />
    <path d="M5 24.400c3-3 6-3 9 0s6 3 9 0" stroke="#c9d7f7" strokeWidth="2.400" strokeLinecap="round" fill="none" />
  </>);
export const IconSrcEntsoe = mark("src-entsoe",
  <>
    <circle cx="16" cy="16" r="10.500" fill="#c9d7f7" stroke="#6f8fe0" strokeWidth="1.600" />
    <path d="M16 7l1.600 4.400 4.600.2-3.600 2.900 1.300 4.500L16 16.400 12.100 19l1.300-4.500-3.600-2.900 4.600-.2z" fill="#f2cf8a" />
  </>);
export const IconSrcOns = mark("src-ons",
  <>
    <path d="M5 24l5-9 4.500 5 4-10 4.500 14z" fill="#b9e3d3" stroke="#4fae8f" strokeWidth="1.600" strokeLinejoin="round" />
    <circle cx="23.500" cy="8.500" r="3" fill="#f2cf8a" />
  </>);
export const IconSrcEia = mark("src-eia",
  <>
    <path d="M16 4.600 4.800 10h22.400z" fill="#c9d7f7" stroke="#6f8fe0" strokeWidth="1.600" />
    <path d="M8 12.600v10M13 12.600v10M19 12.600v10M24 12.600v10" stroke="#8db4ee" strokeWidth="2.400" strokeLinecap="round" />
    <path d="M5 25.600h22" stroke="#6f8fe0" strokeWidth="2.200" strokeLinecap="round" />
  </>);
export const IconSrcEmber = mark("src-ember",
  <>
    <path d="M16 3.400c1.200 4.400-4.800 6.600-4.800 12.200a4.800 4.800 0 0 0 9.600 0c0-2-.8-3.200-1.800-4.400-.4 1.800-1.200 2.600-2 2.800.8-4-.2-7.600-1-10.600z" fill="#f6c3cc" stroke="#e08b98" strokeWidth="1.600" strokeLinejoin="round" />
    <path d="M10 26.800h12" stroke="#f2cf8a" strokeWidth="2.400" strokeLinecap="round" />
  </>);
export const IconSrcWattTime = mark("src-watttime",
  <>
    <circle cx="16" cy="16" r="11" fill="#f1ecfc" stroke="#b8a9ee" strokeWidth="1.800" />
    <path d="M8.600 16h3.200l2-5.600 3.400 11 2-5.400h3.200" stroke="#8a78d8" strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round" />
  </>);
export const IconSrcUkGrid = mark("src-uk-grid",
  <>
    <path d="M16 4c-5.600 3.600-8 8-8 12.400a8 8 0 0 0 16 0C24 12 21.600 7.600 16 4z" fill="#e3f4ee" stroke="#6fc3a8" strokeWidth="1.800" strokeLinejoin="round" />
    <path d="M16 9v18M16 17l-4-3M16 20l4-3" stroke="#2f8f78" strokeWidth="1.600" strokeLinecap="round" />
  </>);
