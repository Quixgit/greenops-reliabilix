import { describe, expect, it } from "vitest";
import { createLimiter, loginUrl, safeReturnTo, validEmail } from "./login";

describe("safeReturnTo", () => {
  it("keeps app paths and refuses everything else", () => {
    expect(safeReturnTo("/projects")).toBe("/projects");
    for (const bad of ["https://evil.example", "//evil.example", "/\\evil", "javascript:alert(1)", "/auth/logout", "/login", "", undefined, 5]) {
      expect(safeReturnTo(bad)).toBe("/overview");
    }
  });
});

describe("loginUrl", () => {
  it("adds a provider and a valid e-mail hint only", () => {
    expect(loginUrl({ connection: "github" })).toBe("/auth/login?connection=github&returnTo=%2Foverview");
    expect(loginUrl({ email: "a@b.io" })).toContain("login_hint=a%40b.io");
    expect(loginUrl({ email: "not an email" })).not.toContain("login_hint");
  });
});

describe("validEmail", () => {
  it("accepts addresses and rejects junk", () => {
    expect(validEmail("jo@corp.example")).toBe(true);
    expect(validEmail("jo@corp")).toBe(false);
    expect(validEmail("a b@c.de")).toBe(false);
    expect(validEmail(null)).toBe(false);
  });
});

describe("createLimiter", () => {
  it("allows max hits per window, then refuses, then recovers", () => {
    let t = 0;
    const allow = createLimiter(2, 1000, () => t);
    expect([allow("ip"), allow("ip"), allow("ip")]).toEqual([true, true, false]);
    expect(allow("other")).toBe(true);
    t = 1001;
    expect(allow("ip")).toBe(true);
  });
});
