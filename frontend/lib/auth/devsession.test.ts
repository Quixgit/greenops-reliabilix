import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { credentialsOk, devLoginEnabled, sessionOk, sessionValue } from "./devsession";

const KEYS = ["DEV_AUTH", "APP_ENV", "DEV_BASIC_AUTH", "DEV_LOGIN_PAGE"] as const;
let saved: Record<string, string | undefined> = {};

beforeEach(() => {
  saved = Object.fromEntries(KEYS.map((k) => [k, process.env[k]]));
  Object.assign(process.env, { DEV_AUTH: "true", APP_ENV: "dev", DEV_BASIC_AUTH: "admin:s3cret", DEV_LOGIN_PAGE: "true" });
});
afterEach(() => {
  for (const k of KEYS) {
    if (saved[k] === undefined) delete process.env[k];
    else process.env[k] = saved[k];
  }
});

describe("development sign-in", () => {
  it("is on only with dev auth, a password and the page flag", () => {
    expect(devLoginEnabled()).toBe(true);
    process.env.DEV_LOGIN_PAGE = "false";
    expect(devLoginEnabled()).toBe(false);
    process.env.DEV_LOGIN_PAGE = "true";
    process.env.APP_ENV = "production";
    expect(devLoginEnabled()).toBe(false);
  });

  it("checks the credentials exactly", () => {
    expect(credentialsOk("admin", "s3cret")).toBe(true);
    expect(credentialsOk("admin", "S3cret")).toBe(false);
    expect(credentialsOk("root", "s3cret")).toBe(false);
    expect(credentialsOk("", "")).toBe(false);
  });

  it("accepts only its own signed cookie", async () => {
    const v = await sessionValue();
    expect(await sessionOk(v)).toBe(true);
    expect(await sessionOk(undefined)).toBe(false);
    expect(await sessionOk("1")).toBe(false);
    expect(await sessionOk(v.slice(0, -1) + (v.endsWith("0") ? "1" : "0"))).toBe(false);
    process.env.DEV_BASIC_AUTH = "admin:other";
    expect(await sessionOk(v)).toBe(false); // a cookie signed under another password is void
  });
});
