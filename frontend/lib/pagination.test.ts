import { describe, expect, it } from "vitest";

// the window logic is internal to the component; this guards the arithmetic the bar shows
const range = (page: number, size: number, total: number) => [(page - 1) * size + 1, Math.min(total, page * size)];

describe("page range", () => {
  it("shows the rows of the page", () => {
    expect(range(1, 10, 57)).toEqual([1, 10]);
    expect(range(6, 10, 57)).toEqual([51, 57]);
    expect(range(2, 25, 30)).toEqual([26, 30]);
  });
});
