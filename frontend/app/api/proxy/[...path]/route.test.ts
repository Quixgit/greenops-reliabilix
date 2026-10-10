import { describe, expect, it, vi } from "vitest";

vi.mock("@/lib/auth/session", () => ({ getApiToken: async () => "token" }));

// The screens save settings, policies and lists with PUT. The proxy once exported no PUT handler, so every such
// save answered "Method Not Allowed" in the browser while the API itself worked.
describe("API proxy", () => {
  it("exports a handler for every method the screens use", async () => {
    const route = await import("./route");
    for (const m of ["GET", "POST", "PUT", "PATCH", "DELETE"] as const) expect(typeof route[m], m).toBe("function");
  });
});
