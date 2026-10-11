import pkg from "../package.json";

export type Stage = "alpha" | "beta" | "rc" | "stable" | "dev";

/** "0.1.0-alpha.90" -> { number: "0.1.0", stage: "alpha", full: "0.1.0-alpha.90" }. See docs/release/versioning.md. */
export function parseVersion(v: string): { number: string; stage: Stage; full: string } {
  const [number, pre] = v.split("-", 2);
  const id = pre?.split(".")[0];
  const stage: Stage = !pre ? (v === "dev" ? "dev" : "stable") : id === "alpha" || id === "beta" || id === "rc" ? id : "dev";
  return { number: v === "dev" ? "dev" : number, stage, full: v };
}

/** The version this build was made from: injected by the image build, else the one in package.json. */
export const BUILD_VERSION = parseVersion(process.env.NEXT_PUBLIC_BUILD_VERSION && process.env.NEXT_PUBLIC_BUILD_VERSION !== "dev" ? process.env.NEXT_PUBLIC_BUILD_VERSION : pkg.version);
