"use client";

import { useEffect, useRef } from "react";
import { echarts } from "@/lib/echarts";
import type { EChartsCoreOption } from "echarts/core";

const prefersReduced = () => typeof window !== "undefined" && !!window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;

/** Thin ECharts wrapper: init once, update on option change, resize with the container, dispose on unmount. */
export function Chart({ option, height, ariaLabel, onReady }: { option: EChartsCoreOption; height: number; ariaLabel: string; onReady?: () => void }) {
  const ref = useRef<HTMLDivElement>(null);
  const inst = useRef<ReturnType<typeof echarts.init> | null>(null);

  useEffect(() => {
    if (!ref.current) return;
    onReady?.();
    const chart = echarts.init(ref.current, undefined, { renderer: "canvas" });
    inst.current = chart;
    const ro = new ResizeObserver(() => chart.resize());
    ro.observe(ref.current);
    return () => { ro.disconnect(); chart.dispose(); inst.current = null; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Smooth by default: lines draw in, slices expand, updates morph. Reduced-motion users get a static chart.
  useEffect(() => {
    inst.current?.setOption({ animation: !prefersReduced(), animationDuration: 1100, animationEasing: "cubicOut", animationDurationUpdate: 600, animationEasingUpdate: "cubicInOut", ...option }, true);
  }, [option]);

  return <div ref={ref} role="img" aria-label={ariaLabel} style={{ height, width: "100%" }} />;
}
