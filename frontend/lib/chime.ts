/** A short, soft two-note chime made with the Web Audio API (no audio file). Browsers allow it only after the person has used the page. */
export function playChime() {
  try {
    const AC = window.AudioContext ?? (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
    const ctx = new AC();
    const now = ctx.currentTime;
    [[784, 0], [1047, 0.12]].forEach(([f, d]) => {
      const o = ctx.createOscillator(), g = ctx.createGain();
      o.type = "sine"; o.frequency.value = f;
      g.gain.setValueAtTime(0, now + d);
      g.gain.linearRampToValueAtTime(0.12, now + d + 0.02);
      g.gain.exponentialRampToValueAtTime(0.0001, now + d + 0.35);
      o.connect(g).connect(ctx.destination);
      o.start(now + d); o.stop(now + d + 0.4);
    });
    window.setTimeout(() => void ctx.close(), 900);
  } catch { /* no audio available */ }
}

const KEY = "rlx.sound";
/** Personal choice: "on" / "off" / null (follow the organization's default). */
export const readSoundPref = (): "on" | "off" | null => { try { const v = localStorage.getItem(KEY); return v === "on" || v === "off" ? v : null; } catch { return null; } };
export const writeSoundPref = (v: "on" | "off") => { try { localStorage.setItem(KEY, v); } catch { /* ignore */ } };
