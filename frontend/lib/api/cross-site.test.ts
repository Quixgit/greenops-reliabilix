import { describe, expect, it } from "vitest";
import { crossSite } from "./cross-site";

const h = (o: Record<string, string>) => new Headers(o);

describe("crossSite", () => {
  it("lets reads through whatever their origin", () => {
    expect(crossSite("GET", h({ "sec-fetch-site": "cross-site" }), ["app.example"])).toBe(false);
  });
  it("refuses a change that a browser says came from another site", () => {
    expect(crossSite("POST", h({ "sec-fetch-site": "cross-site" }), ["app.example"])).toBe(true);
    expect(crossSite("DELETE", h({ "sec-fetch-site": "same-site" }), ["app.example"])).toBe(true);
  });
  it("refuses a change whose Origin is not this host", () => {
    expect(crossSite("PATCH", h({ origin: "https://evil.example" }), ["app.example"])).toBe(true);
    expect(crossSite("POST", h({ origin: "not a url" }), ["app.example"])).toBe(true);
  });
  it("knows every name the site goes by (a proxy may hide the public one from the host header)", () => {
    expect(crossSite("POST", h({ origin: "https://app.example.com" }), ["10.0.0.5:3000", "app.example.com"])).toBe(false);
    expect(crossSite("POST", h({ origin: "https://evil.example" }), ["10.0.0.5:3000", "app.example.com", ""])).toBe(true);
  });
  it("accepts the site's own pages and clients that are not browsers", () => {
    expect(crossSite("POST", h({ "sec-fetch-site": "same-origin", origin: "https://app.example" }), ["app.example"])).toBe(false);
    expect(crossSite("PUT", h({ origin: "http://23.19.228.158:38451" }), ["23.19.228.158:38451"])).toBe(false);
    expect(crossSite("POST", h({}), ["app.example"])).toBe(false);
  });
});
