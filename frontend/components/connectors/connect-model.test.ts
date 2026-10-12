import { describe, expect, it } from "vitest";
import { verifyHint } from "./connect-model";
import { snippets } from "./setup-as-code";

describe("verifyHint", () => {
  it("explains the usual failures in plain words", () => {
    expect(verifyHint("AWS denied the request (AccessDenied): check that the role has the read permissions")).toBe("cw.hint.denied");
    expect(verifyHint("the billing export table was not found")).toBe("cw.hint.not_found");
    expect(verifyHint("the export has no data yet")).toBe("cw.hint.no_data");
    expect(verifyHint("request timed out")).toBe("cw.hint.network");
    expect(verifyHint("something else entirely")).toBeNull();
  });
});

describe("setup as code", () => {
  it("fills in what the person typed and leaves a placeholder otherwise", () => {
    const gcp = snippets("gcp", { account: "acme-prod-123", table: "bq://acme-billing/billing_export/gcp_billing_export_v1_X" });
    expect(gcp.find((s) => s.id === "cli")!.code).toContain("acme-billing:billing_export");
    expect(snippets("azure", { account: "", table: "" })[1].code).toContain("<subscription id>");
    expect(snippets("aws", { account: "", table: "" })[0].code).toContain("ce:GetCostAndUsage");
  });
  it("never asks for more than read access", () => {
    for (const p of ["aws", "azure", "gcp"] as const) {
      const all = snippets(p, { account: "x", table: "" }).map((s) => s.code).join("\n");
      expect(all).not.toMatch(/Owner|Contributor|AdministratorAccess|"\*:\*"|roles\/editor/);
    }
  });
});
