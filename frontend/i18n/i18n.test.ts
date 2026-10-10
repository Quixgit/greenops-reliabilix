import { describe, expect, it } from "vitest";
import { negotiate } from "./config";
import en from "./messages/en.json";
import ru from "./messages/ru.json";
import { placeholders, translate } from "./translate";

describe("catalogs", () => {
  it("Russian has every English key and no others (plural forms excepted)", () => {
    const base = (k: string) => k.replace(/\.(zero|one|two|few|many|other)$/, "");
    const enKeys = new Set(Object.keys(en).map(base)), ruKeys = new Set(Object.keys(ru).map(base));
    expect([...enKeys].filter((k) => !ruKeys.has(k))).toEqual([]);
    expect([...ruKeys].filter((k) => !enKeys.has(k))).toEqual([]);
  });
  it("keeps the placeholders of every message", () => {
    for (const [k, v] of Object.entries(ru)) {
      const baseKey = k in en ? k : Object.keys(en).find((e) => e.startsWith(k.replace(/\.(one|few|many|other)$/, "") + "."));
      if (baseKey) expect(placeholders(v), k).toEqual(placeholders((en as Record<string, string>)[baseKey]));
    }
  });
});

describe("translate", () => {
  const m = ru as Record<string, string>, f = en as Record<string, string>;
  it("falls back to English, then to the given fallback, then to the key", () => {
    expect(translate({}, f, "ru", "nav.overview")).toBe("Overview");
    expect(translate({}, {}, "ru", "no.such", undefined, "Fallback")).toBe("Fallback");
    expect(translate({}, {}, "ru", "no.such")).toBe("no.such");
  });
  it("fills placeholders and keeps unknown ones visible", () => {
    expect(translate(m, f, "ru", "carbon.days_with_data", { days: 5, window: 30 })).toBe("Показания есть за 5 из 30 дн.");
    expect(translate({ a: "Hi {name} {x}" }, {}, "en", "a", { name: "Ann" })).toBe("Hi Ann {x}");
  });
  it("chooses the Russian plural form", () => {
    const n = (c: number) => translate(m, f, "ru", "carbon.regions", { count: c });
    expect([n(1), n(2), n(5), n(21)]).toEqual(["1 регион", "2 региона", "5 регионов", "21 регион"]);
  });
});

describe("negotiate", () => {
  it("picks by quality and primary subtag among the offered languages", () => {
    expect(negotiate("ru-RU,ru;q=0.9,en;q=0.8")).toBe("ru");
    expect(negotiate("fr-FR,fr;q=0.9")).toBeUndefined();
    expect(negotiate("en;q=0.3, ru;q=0.9")).toBe("ru");
    expect(negotiate(null)).toBeUndefined();
    expect(negotiate("ru,en;q=0.5", ["en"])).toBe("en"); // only offered languages count
  });
});
