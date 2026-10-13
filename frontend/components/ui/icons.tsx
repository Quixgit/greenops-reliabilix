import type { LucideIcon } from "lucide-react";
import type { ComponentType } from "react";
import { cn } from "@/lib/utils";

/**
 * The platform's icon family (DX): one 24-unit grid, a 1.6 stroke, square ends and mitred joins, geometric shapes, no fills and no second
 * tone. Every icon takes the usual `className` and `strokeWidth` and follows the text colour. The cloud and data-source marks are flat
 * squares with their initials, so no vendor logo clashes with the rest.
 */
export type IconProps = { className?: string; strokeWidth?: number; "aria-label"?: string };
export type IconType = ComponentType<IconProps>;
/** Either set: the screens still holding a stock glyph keep working while the drawn set takes over. */
export type AnyIcon = LucideIcon | IconType;

function make(name: string, d: string): IconType {
  const Icon = ({ className, strokeWidth = 1.6, "aria-label": label }: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={strokeWidth} strokeLinecap="square" strokeLinejoin="miter" className={cn("size-5 shrink-0", className)} aria-hidden={label ? undefined : true} aria-label={label} role={label ? "img" : undefined}>
      <path d={d} />
    </svg>
  );
  Icon.displayName = `Icon(${name})`;
  return Icon;
}

/** A flat square mark with initials: a cloud or a data source, in its own colour. */
export function mark(name: string, letters: string, bg: string, ink: string): IconType {
  const Icon = ({ className }: IconProps) => (
    <svg viewBox="0 0 32 32" className={cn("size-10 shrink-0", className)} aria-hidden>
      <rect width="32" height="32" fill={bg} />
      <text x="16" y="21" textAnchor="middle" fontSize={letters.length > 2 ? 11 : 13} fontWeight="700" fill={ink} fontFamily="Inter, system-ui, sans-serif">{letters}</text>
    </svg>
  );
  Icon.displayName = `Icon(${name})`;
  return Icon;
}

export const IconOverview = make("overview", "M3 3h8v9H3z M13 3h8v5h-8z M13 10h8v11h-8z M3 14h8v7H3z");
export const IconFolder = make("folder", "M3 5h7l2 3h9v12H3z M3 12h18");
export const IconCluster = make("cluster", "M12 2.5l8.5 4.7v9.6L12 21.5l-8.5-4.7V7.2z M12 7.5v9 M8 9.8l8 4.4 M16 9.8l-8 4.4");
export const IconLeaf = make("leaf", "M4 20v-8l8-8h8v8l-8 8z M4 20L14 10");
export const IconReport = make("report", "M5 3h10l4 4v14H5z M15 3v4h4 M8 11h8 M8 14h8 M8 17h5");
export const IconBulb = make("bulb", "M12 2.5l6.5 4v7l-3 2.5V19h-7v-3L5.5 13.5v-7z M9.5 22h5");
export const IconFlow = make("flow", "M3 4h8v6H3z M13 14h8v6h-8z M7 10v7h6 M17 14V9h-3");
export const IconBranch = make("branch", "M4 3h5v5H4z M4 16h5v5H4z M15 3h5v5h-5z M6.5 8v8 M17.5 8v3.5H6.5");
export const IconPlug = make("plug", "M9 3v5 M15 3v5 M6 8h12v6l-4 4h-4l-4-4z M12 18v3");
export const IconBell = make("bell", "M12 3l6.5 4v6l2 3.5h-17l2-3.5V7z M9.5 20h5");
export const IconSliders = make("sliders", "M3 7h9 M16 7h5 M3 17h4 M11 17h10 M12 4h4v6h-4z M7 14h4v6H7z");
export const IconShield = make("shield", "M12 2.5l8 3v7c0 5-3.5 8-8 9-4.5-1-8-4-8-9v-7z");
export const IconCoin = make("coin", "M8 3h8l5 5v8l-5 5H8l-5-5V8z M12 7v10 M9 10h5 M9 14h5");
export const IconCloud = make("cloud", "M6 18h12l3.5-3.5-2-4.5h-4L12.5 6 8 9.5l-4 1.5z");
export const IconBolt = make("bolt", "M13.5 2L5 14h6l-1.5 8L19 10h-6.5z");
export const IconGauge = make("gauge", "M3 18l2.5-7.5L12 6l6.5 4.5L21 18z M12 18l4-6");
export const IconUsers = make("users", "M3 21v-3l3-2h6l3 2v3z M6 7h6v6H6z M16 6h4v5h-4z M17 15l4 2v4h-4");
export const IconKey = make("key", "M3 8h7v7H3z M10 11.5h11 M17 11.5V16 M21 11.5V14");
export const IconLock = make("lock", "M5 11h14v10H5z M8 11V7l4-4 4 4v4 M12 15v3");
export const IconGlobe = make("globe", "M3 3h18v18H3z M3 12h18 M12 3v18 M3 7.5h18 M3 16.5h18");
export const IconChart = make("chart", "M3 3v18h18 M7 17v-5 M12 17V8 M17 17v-8");
export const IconTrend = make("trend", "M3 3v18h18 M6 16l4-5 3 3 6-8");
export const IconWallet = make("wallet", "M3 6h18v14H3z M3 10.5h18 M15 15h3");
export const IconMail = make("mail", "M3 5h18v14H3z M3 6l9 7 9-7");
export const IconClock = make("clock", "M3 3h18v18H3z M12 7v5.5l4 2");
export const IconDatabase = make("database", "M5 4h14v4H5z M5 10h14v4H5z M5 16h14v4H5z");
export const IconTarget = make("target", "M3 3h18v18H3z M7 7h10v10H7z M11 11h2v2h-2z");
export const IconPin = make("pin", "M12 21.5l-6-8V3.5h12v10z M10 7.5h4v4h-4z");
export const IconWarning = make("warning", "M12 3l10 18H2z M12 10v5 M12 17.5v.5");
export const IconCheck = make("check", "M4 12.5l5 5L20 6.5");
export const IconClipboard = make("clipboard", "M5 4h14v17H5z M9 2h6v4H9z M8 11h8 M8 15h8");
export const IconBuilding = make("building", "M5 21V3h10v18 M15 9h4v12 M8 7h4 M8 11h4 M8 15h4 M3 21h18");
export const IconSend = make("send", "M3 11l18-8-8 18-2-8z");
export const IconPlay = make("play", "M7 4l13 8-13 8z");
export const IconRefresh = make("refresh", "M20 11a8 8 0 0 0-14.5-3.5 M4 3.5v4h4 M4 13a8 8 0 0 0 14.5 3.5 M20 20.5v-4h-4");
export const IconPlus = make("plus", "M12 4v16 M4 12h16");
export const IconArrowRight = make("arrowright", "M4 12h16 M14 6l6 6-6 6");
export const IconChevronRight = make("chevronright", "M9 5l7 7-7 7");
export const IconInfo = make("info", "M4 4h16v16H4z M12 11v6 M12 7v1.2");
export const IconClose = make("close", "M5 5l14 14 M19 5L5 19");
export const IconSearch = make("search", "M4 4h11v11H4z M15 15l5.5 5.5");
export const IconServer = make("server", "M3 4h18v6H3z M3 14h18v6H3z M7 7h2 M7 17h2");
export const IconNetwork = make("network", "M10 3h4v4h-4z M3 17h4v4H3z M17 17h4v4h-4z M12 7v5 M5 17v-5h14v5");
export const IconShare = make("share", "M16 3h5v5h-5z M3 10h5v5H3z M16 16h5v5h-5z M8 12l8-7 M8 13l8 6");
export const IconPalette = make("palette", "M3 3h18v18H3z M7 7h4v4H7z M13 7h4v4h-4z M7 13h4v4H7z");
export const IconEye = make("eye", "M2 12l4-5h12l4 5-4 5H6z M10 10h4v4h-4z");
export const IconEyeOff = make("eyeoff", "M2 12l4-5h12l4 5-4 5H6z M4 20L20 4");
export const IconCable = make("cable", "M7 3v6 M17 3v6 M5 9h14v4l-4 4h-6l-4-4z M12 17v4");
export const IconChevronDown = make("chevrondown", "M5 9l7 7 7-7");
export const IconChevronLeft = make("chevronleft", "M15 5l-7 7 7 7");
export const IconChevronsUpDown = make("chevronsupdown", "M7 9l5-5 5 5 M7 15l5 5 5-5");
export const IconArrowUp = make("arrowup", "M12 19V5 M6 11l6-6 6 6");
export const IconArrowDown = make("arrowdown", "M12 5v14 M6 13l6 6 6-6");
export const IconArrowLeft = make("arrowleft", "M20 12H4 M10 6l-6 6 6 6");
export const IconArrowUpRight = make("arrowupright", "M7 17L17 7 M8 7h9v9");
export const IconMinus = make("minus", "M4 12h16");
export const IconPencil = make("pencil", "M4 20l1-5L16 4l4 4L9 19z M14 6l4 4");
export const IconTrash = make("trash", "M4 6h16 M9 6V3h6v3 M6 6v15h12V6 M10 10v7 M14 10v7");
export const IconCard = make("card", "M3 5h18v14H3z M3 10h18 M6 15h4");
export const IconRepeat = make("repeat", "M4 8h13 M13 4l4 4-4 4 M20 16H7 M11 12l-4 4 4 4");
export const IconHistory = make("history", "M3 12a9 9 0 1 0 3-6.7 M3 4v5h5 M12 7v5l3 2");
export const IconUndo = make("undo", "M4 9h11a5 5 0 0 1 0 10H9 M8 5L4 9l4 4");
export const IconPower = make("power", "M12 3v9 M6 6.5a8 8 0 1 0 12 0");
export const IconUserPlus = make("userplus", "M3 21v-4l4-3h6l4 3v4z M7 4h6v6H7z M19 8v6 M16 11h6");
export const IconUser = make("user", "M4 21v-4l4-3h8l4 3v4z M9 4h6v6H9z");
export const IconDownload = make("download", "M12 3v12 M6.5 10l5.5 5.5 5.5-5.5 M4 21h16");
export const IconCopy = make("copy", "M8 8h12v13H8z M4 16V3h12");
export const IconEnter = make("enter", "M10 4h10v16H10 M3 12h12 M10 7l5 5-5 5");
export const IconCheckCircle = make("checkcircle", "M3 3h18v18H3z M7.5 12.5l3 3 6-7");
export const IconCircleDashed = make("circledashed", "M3 3h4 M10 3h4 M17 3h4 M21 7v4 M21 14v4 M17 21h4 M10 21h4 M3 21h4 M3 14v4 M3 7v4");
export const IconHourglass = make("hourglass", "M6 3h12 M6 21h12 M7 3v5l5 4-5 4v5 M17 3v5l-5 4 5 4v5");
export const IconCpu = make("cpu", "M7 7h10v10H7z M10 3v4 M14 3v4 M10 17v4 M14 17v4 M3 10h4 M3 14h4 M17 10h4 M17 14h4");
export const IconLogOut = make("logout", "M10 4H4v16h6 M14 8l5 4-5 4 M19 12H9");
export const IconMenu = make("menu", "M3 6h18 M3 12h18 M3 18h18");
export const IconLanguages = make("languages", "M3 4h10v10H3z M6 7h4 M8 7v3 M12 20l3-7 3 7 M13 18h4");
export const IconCalculator = make("calculator", "M5 3h14v18H5z M8 6h8v4H8z M8 13h2 M12 13h2 M8 17h2 M12 17h2 M16 13v4");
export const IconDrive = make("drive", "M3 14h18v6H3z M3 14l3-9h12l3 9 M7 17h2");
export const IconTable = make("table", "M3 4h18v16H3z M3 10h18 M3 15h18 M9 4v16");
export const IconLocate = make("locate", "M12 3v4 M12 17v4 M3 12h4 M17 12h4 M7 7h10v10H7z M11 11h2v2h-2z");
export const IconExpand = make("expand", "M4 9V4h5 M20 9V4h-5 M4 15v5h5 M20 15v5h-5");
export const IconSparkles = make("sparkles", "M12 3l2 6 6 2-6 2-2 6-2-6-6-2 6-2z M19 3v4 M17 5h4");
export const IconReceipt = make("receipt", "M5 3h14v18l-3-2-2 2-2-2-2 2-2-2-3 2z M8 8h8 M8 12h8");
export const IconTimer = make("timer", "M9 2h6 M3 5h18v16H3z M12 9v5l3 2");
export const IconMerge = make("merge", "M5 3v7l7 5 7-5V3 M12 15v6");
export const IconLoader = make("loader", "M12 3v4 M12 17v4 M3 12h4 M17 12h4 M5.5 5.5l3 3 M15.5 15.5l3 3 M18.5 5.5l-3 3 M8.5 15.5l-3 3");
export const IconCostCarbon = make("costcarbon", "M12 2.5l9.5 9.5-9.5 9.5L2.5 12z M12 8v8 M9 11h6");
export const IconGridMeasure = make("gridmeasure", "M3 18l2.5-7.5L12 6l6.5 4.5L21 18z M3 18h18 M12 18l4-6");
export const IconInsightAction = make("insightaction", "M3 3v18h18 M6 16l4-5 3 3 6-8 M15 6h4v4");
export const IconTrust = make("trust", "M12 2.5l8 3v7c0 5-3.5 8-8 9-4.5-1-8-4-8-9v-7z M8.5 12l2.5 3 4.5-5");

export const IconAws = mark("aws", "AWS", "#d9a66b", "#ffffff");
export const IconAzure = mark("azure", "Az", "#6f8fbf", "#ffffff");
export const IconGcp = mark("gcp", "GCP", "#c98b8b", "#ffffff");
export const IconKubernetes = mark("kubernetes", "K8s", "#4d6a9a", "#ffffff");
export const IconDigitalOcean = mark("digitalocean", "DO", "#6f9fb8", "#ffffff");
export const IconSrcElectricityMaps = mark("srcelectricitymaps", "EM", "#d7e3d3", "#3f6a4a");
export const IconSrcEnergyCharts = mark("srcenergycharts", "EC", "#d9e2ee", "#2c4b78");
export const IconSrcIeso = mark("srcieso", "IE", "#d9e2ee", "#2c4b78");
export const IconSrcHydroQuebec = mark("srchydroquebec", "HQ", "#d9e2ee", "#2c4b78");
export const IconSrcEntsoe = mark("srcentsoe", "EN", "#eadfca", "#6e5a33");
export const IconSrcOns = mark("srcons", "ON", "#d7e3d3", "#3f6a4a");
export const IconSrcEia = mark("srceia", "EI", "#eadfca", "#6e5a33");
export const IconSrcEmber = mark("srcember", "EB", "#f3dac8", "#8a4d27");
export const IconSrcWattTime = mark("srcwatttime", "WT", "#ecd5d2", "#8a403d");
export const IconSrcUkGrid = mark("srcukgrid", "UK", "#d7e3d3", "#3f6a4a");
