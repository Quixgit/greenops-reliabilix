import { describe, expect, it } from "vitest";
import { band } from "./carbon-board";

describe("grid bands", () => {
  it("follows the platform thresholds", () => {
    expect(band(0).key).toBe("clean");
    expect(band(149.9).key).toBe("clean");
    expect(band(150).key).toBe("moderate");
    expect(band(299).key).toBe("moderate");
    expect(band(300).key).toBe("high");
    expect(band(449).key).toBe("high");
    expect(band(450).key).toBe("very_high");
  });
});
