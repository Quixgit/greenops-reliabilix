import { describe, expect, it } from "vitest";
import { basicAuthOk } from "./basic";

const hdr = (s: string) => `Basic ${btoa(s)}`;

describe("basicAuthOk", () => {
  it("accepts exactly the configured credentials", () => {
    expect(basicAuthOk(hdr("demo:s3cret"), "demo:s3cret")).toBe(true);
  });
  it("rejects wrong, partial, longer and missing credentials", () => {
    for (const h of [hdr("demo:s3cre"), hdr("demo:s3cret!"), hdr("other:s3cret"), hdr(""), "Basic !!!not-base64", "Bearer abc", null]) {
      expect(basicAuthOk(h, "demo:s3cret")).toBe(false);
    }
  });
});
