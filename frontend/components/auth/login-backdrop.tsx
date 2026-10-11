"use client";

import { useEffect, useRef } from "react";
import { landDots, project, MAP_H, MAP_W } from "@/lib/dot-grid";
import { REGION_GEO } from "@/lib/regions";

/**
 * The sign-in scene: the same dot-matrix world as the dashboard map, held low in the dark so the form stays the
 * subject. Cloud regions pulse softly and light travels between them along arcs, the way cost and carbon travel
 * through a cloud estate. Decorative only; it stands still for people who asked for reduced motion.
 */
export function LoginBackdrop() {
  const ref = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const cv = ref.current;
    if (!cv) return;
    const ctx = cv.getContext("2d");
    if (!ctx) return;
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const dots = landDots();
    const places = Object.values(REGION_GEO);
    // an even sample of the regions the platform knows: stable between visits, so the scene feels designed, not random
    const picked = places.filter((_, i) => i % Math.max(1, Math.floor(places.length / 34)) === 0).map((p) => project(p.lat, p.lon));
    const arcs = Array.from({ length: Math.min(16, picked.length) }, (_, i) => {
      const a = picked[(i * 7) % picked.length], b = picked[(i * 11 + 5) % picked.length];
      return { a, b, speed: 0.045 + ((i * 37) % 17) / 400, phase: ((i * 53) % 100) / 100, hue: i % 4 };
    });
    const twinkle = Array.from({ length: 70 }, (_, i) => ({ i: (i * 2654435761) % dots.length, p: ((i * 97) % 100) / 100 }));
    const base = document.createElement("canvas");
    let W = 0, H = 0, s = 1, ox = 0, oy = 0, dpr = 1, raf = 0, last = 0;

    const layout = () => {
      dpr = Math.min(2, window.devicePixelRatio || 1);
      W = cv.clientWidth; H = cv.clientHeight;
      cv.width = W * dpr; cv.height = H * dpr; base.width = W * dpr; base.height = H * dpr;
      s = Math.max(W / MAP_W, (H * 0.7) / MAP_H);
      ox = (W - MAP_W * s) / 2; oy = (H - MAP_H * s) / 2 + H * 0.02;
      const b = base.getContext("2d")!;
      b.setTransform(dpr, 0, 0, dpr, 0, 0);
      b.clearRect(0, 0, W, H);
      b.fillStyle = "rgba(15, 118, 110, 0.28)";
      const r = Math.max(0.8, 0.5 * s);
      b.beginPath();
      for (const d of dots) { const x = ox + d.x * s, y = oy + d.y * s; b.moveTo(x + r, y); b.arc(x, y, r, 0, 6.2832); }
      b.fill();
    };

    const COLORS = ["13,148,136", "37,99,235", "101,163,13", "124,58,237"];
    const frame = (now: number) => {
      raf = requestAnimationFrame(frame);
      if (now - last < 33) return; // ~30 fps is plenty for a backdrop
      last = now;
      const t = now / 1000;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, W, H);
      ctx.drawImage(base, 0, 0, W, H);
      const X = (v: number) => ox + v * s, Y = (v: number) => oy + v * s;
      // a few land dots catch the light and fade
      for (const w of twinkle) {
        const d = dots[w.i]; if (!d) continue;
        const a = 0.5 + 0.5 * Math.sin(t * 0.9 + w.p * 6.28);
        ctx.fillStyle = `rgba(13,148,136,${(0.10 + 0.40 * a).toFixed(3)})`;
        ctx.beginPath(); ctx.arc(X(d.x), Y(d.y), Math.max(1.1, 0.8 * s), 0, 6.2832); ctx.fill();
      }
      // arcs and the light that travels along them
      for (const a of arcs) {
        const x1 = X(a.a[0]), y1 = Y(a.a[1]), x2 = X(a.b[0]), y2 = Y(a.b[1]);
        const mx = (x1 + x2) / 2, my = (y1 + y2) / 2 - Math.hypot(x2 - x1, y2 - y1) * 0.28;
        const c = COLORS[a.hue];
        ctx.strokeStyle = `rgba(${c},0.16)`; ctx.lineWidth = 1;
        ctx.beginPath(); ctx.moveTo(x1, y1); ctx.quadraticCurveTo(mx, my, x2, y2); ctx.stroke();
        const p = (t * a.speed + a.phase) % 1;
        const q = (k: number) => { const u = 1 - k; return [u * u * x1 + 2 * u * k * mx + k * k * x2, u * u * y1 + 2 * u * k * my + k * k * y2]; };
        for (let k = 0; k < 6; k++) {
          const [px, py] = q(Math.max(0, p - k * 0.012));
          ctx.fillStyle = `rgba(${c},${(0.55 - k * 0.09).toFixed(2)})`;
          ctx.beginPath(); ctx.arc(px, py, Math.max(1, (1.9 - k * 0.2) * Math.min(1.4, s / 3)), 0, 6.2832); ctx.fill();
        }
      }
      // regions: a steady point and a ring that expands and fades
      picked.forEach((m, i) => {
        const x = X(m[0]), y = Y(m[1]);
        const ph = (t * 0.35 + i * 0.173) % 1;
        const c = COLORS[i % 4];
        ctx.strokeStyle = `rgba(${c},${((1 - ph) * 0.38).toFixed(3)})`; ctx.lineWidth = 1.2;
        ctx.beginPath(); ctx.arc(x, y, 2 + ph * 15 * Math.min(1.5, s / 3), 0, 6.2832); ctx.stroke();
        ctx.fillStyle = `rgba(${c},0.7)`;
        ctx.beginPath(); ctx.arc(x, y, Math.max(1.6, 1.2 * Math.min(1.5, s / 3)), 0, 6.2832); ctx.fill();
      });
    };

    layout();
    if (reduce) frame(1000); else raf = requestAnimationFrame(frame);
    const onResize = () => { layout(); if (reduce) frame(1000); };
    window.addEventListener("resize", onResize);
    return () => { cancelAnimationFrame(raf); window.removeEventListener("resize", onResize); };
  }, []);

  return (
    <div aria-hidden className="pointer-events-none absolute inset-0 overflow-hidden bg-[#f3f8f7]">
      <div className="aurora aurora-a absolute -left-[10%] -top-[25%] h-[75%] w-[60%] rounded-full bg-[radial-gradient(closest-side,rgba(110,231,183,0.55),transparent)] blur-3xl" />
      <div className="aurora aurora-b absolute -right-[12%] top-[5%] h-[75%] w-[55%] rounded-full bg-[radial-gradient(closest-side,rgba(125,211,252,0.50),transparent)] blur-3xl" />
      <div className="aurora aurora-c absolute bottom-[-30%] left-[15%] h-[65%] w-[65%] rounded-full bg-[radial-gradient(closest-side,rgba(196,181,253,0.40),transparent)] blur-3xl" />
      <canvas ref={ref} className="absolute inset-0 size-full" />
      {/* keeps the words on the left and the form on the right calm and readable */}
      <div className="absolute inset-0 bg-[linear-gradient(90deg,rgba(243,248,247,0.88)_0%,rgba(243,248,247,0.45)_46%,rgba(243,248,247,0.62)_100%)]" />
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_50%_115%,rgba(243,248,247,0.95),transparent_55%)]" />
    </div>
  );
}
