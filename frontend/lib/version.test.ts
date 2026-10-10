import { describe, expect, it } from "vitest";
import { parseVersion } from "./version";

describe("parseVersion", () => {
  it("splits the number and the release stage", () => {
    expect(parseVersion("0.1.0-alpha.9")).toEqual({ number: "0.1.0", stage: "alpha", full: "0.1.0-alpha.9" });
    expect(parseVersion("0.2.0-beta.3").stage).toBe("beta");
    expect(parseVersion("1.0.0-rc.2").stage).toBe("rc");
    expect(parseVersion("1.0.0")).toEqual({ number: "1.0.0", stage: "stable", full: "1.0.0" });
    expect(parseVersion("dev").stage).toBe("dev");
    expect(parseVersion("1.0.0-weird").stage).toBe("dev");
  });
});
