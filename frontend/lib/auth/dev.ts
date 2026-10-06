/**
 * Local development without Auth0. Honoured only when BOTH DEV_AUTH=true and APP_ENV=dev.
 * The backend independently refuses dev tokens unless it runs with ENV=dev, so this
 * flag can never grant access to a production API.
 */
export function devAuthEnabled(): boolean {
  return process.env.DEV_AUTH === "true" && process.env.APP_ENV === "dev";
}
