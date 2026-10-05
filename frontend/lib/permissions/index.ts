// UI-level hints only. Authorization is enforced server-side by the Go API on every endpoint.
export type Role = "owner" | "admin" | "engineer" | "viewer" | "billing";

export const canWrite = (r: Role) => r === "owner" || r === "admin";
