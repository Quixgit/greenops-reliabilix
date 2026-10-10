// UI-level hints only. Authorization is enforced server-side by the Go API on every endpoint.
export type Role = "owner" | "admin" | "engineer" | "viewer" | "billing";

export const canWrite = (r: Role) => r === "owner" || r === "admin";

/** Notification destinations: owners and admins change them, engineers can see what is configured. */
export const canSeeNotifications = (r: Role) => r === "owner" || r === "admin" || r === "engineer";
/** Members, invitations and API keys. */
export const canManageTenant = canWrite;
/** Organization settings: everybody who can manage or engineer sees them; only owners and admins change them. */
export const canSeeSettings = canSeeNotifications;
