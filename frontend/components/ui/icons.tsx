import type { LucideIcon } from "lucide-react";
import type { ComponentType, ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * The platform's own icon set. Drawn on a 24-unit grid with one pen (round caps, a slightly loose hand) and a soft
 * second tone behind the main shape, so a screen reads as one illustration instead of a pile of stock glyphs.
 * Every icon takes the usual `className` and `strokeWidth` and follows the text colour.
 */
export type IconProps = { className?: string; strokeWidth?: number };
export type IconType = ComponentType<IconProps>;
/** Either set: the screens still holding a stock glyph keep working while the drawn set takes over. */
export type AnyIcon = LucideIcon | IconType;

function make(name: string, tone: ReactNode, line: ReactNode): IconType {
  const Icon = ({ className, strokeWidth = 1.6 }: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={strokeWidth} strokeLinecap="round" strokeLinejoin="round" className={cn("size-5 shrink-0", className)} aria-hidden>
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
