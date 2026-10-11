/** Plain names for the background jobs, so a list says what a job does and not only how the code calls it. */
const NAMES: Record<string, [string, string]> = {
  "cloudaccounts:sync_aws_account": ["Sync an AWS account", "Reads cost and usage of one customer account."],
  "cloudaccounts:sync_azure_subscription": ["Sync an Azure subscription", "Reads the cost export of one customer subscription."],
  "cloudaccounts:sync_gcp_billing": ["Sync Google Cloud billing", "Reads the billing export of one customer project."],
  "cloudaccounts:sync_all": ["Sync every connection", "Starts a sync for each customer connection."],
  "cloudaccounts:sync_rightsizing": ["Rightsizing findings of one account", "Reads the provider's rightsizing findings."],
  "cloudaccounts:rightsizing_all": ["Rightsizing for every connection", "Starts a rightsizing read for each connection."],
  "kubernetes:sync": ["Kubernetes sync", "Processes what a customer's cluster agent pushed."],
  "carbon:calculate": ["Calculate carbon", "Carbon of one project's usage."],
  "carbon:refresh_grid_intensity": ["Refresh grid intensity", "Reads the current grid carbon intensity of the regions in use."],
  "carbon:refresh_monthly": ["Refresh monthly national figures", "Reads the monthly national intensity of the regions in use."],
  "carbon:recalculate_all": ["Recalculate carbon", "Redoes every project's calculations with the data that exists now."],
  "carbon:recalculate_window": ["Recalculate a window", "Started from the console: redoes calculations over a chosen window."],
  "carbon:auto_load_history": ["Load the past of regions", "Fills gaps in grid data and redoes the affected calculations."],
  "carbon:backfill_intensity": ["Load past grid data", "Started from the console: fetches past readings of some regions."],
  "recommendations:calculate_all": ["Recommendations for everyone", "Recomputes the recommendations of every organization."],
  "recommendations:calculate": ["Recommendations for a project", "Recomputes one project's recommendations."],
  "recommendations:refresh_price_index": ["Refresh the price index", "Relative price per region from the AWS price list."],
  "finops:refresh_fx": ["Refresh exchange rates", "ECB euro reference rates for converting cost between currencies."],
  "notifications:sweep": ["Notification sweep", "Queues due or stuck deliveries again."],
  "notifications:deliver": ["Deliver a notification", "One message to one destination."],
  "notifications:purge": ["Purge old deliveries", "Drops finished deliveries past retention."],
  "alerts:evaluate_all": ["Evaluate alerts", "Budget and connection alerts of every organization."],
  "lifecycle:erase_due": ["Erase due organizations", "Erases organizations whose grace period is over."],
  "licensing:expire": ["Expire subscriptions", "Returns organizations whose paid period ended to the Free plan."],
  "licensing:remind": ["Renewal reminders", "Tells organizations a paid period is ending."],
  "platform:ensure_partitions": ["Create monthly partitions", "Prepares usage and carbon tables for the coming months."],
  "platform:enforce_retention": ["Enforce retention", "Drops usage and carbon partitions past retention."],
  "platform:purge_errors": ["Purge old errors", "Drops resolved and ignored error groups past retention."],
  "platform:purge_operator_log": ["Purge the operator log", "Drops operator access records past retention."],
  "reports:generate": ["Generate a report", "Renders one report file."],
};

export function taskName(type: string): { title: string; what: string } {
  const n = NAMES[type];
  return n ? { title: n[0], what: n[1] } : { title: type, what: "" };
}

const DAYS = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
const two = (n: number) => String(n).padStart(2, "0");

/** A cron line in words for the shapes the scheduler uses; anything else is shown as it is. */
export function cronWords(spec: string): string {
  const f = spec.trim().split(/\s+/);
  if (f.length !== 5) return spec;
  const [min, hour, dom, mon, dow] = f;
  const num = (s: string) => /^\d+$/.test(s);
  if (spec.trim() === "* * * * *") return "Every minute";
  if (num(min) && hour === "*" && dom === "*" && mon === "*" && dow === "*") return `Every hour at :${two(Number(min))}`;
  const every = hour.match(/^\*\/(\d+)$/);
  if (num(min) && every && dom === "*" && mon === "*" && dow === "*") return `Every ${every[1]} hours at :${two(Number(min))}`;
  const hours = hour.split(",");
  if (num(min) && hours.every(num) && dom === "*" && mon === "*") {
    const at = hours.map((h) => `${two(Number(h))}:${two(Number(min))}`).join(" and ");
    if (dow === "*") return hours.length === 1 ? `Every day at ${at}` : `Every day at ${at}`;
    if (num(dow)) return `Every ${DAYS[Number(dow) % 7]} at ${at}`;
  }
  return spec;
}

/** "in 5 min", "in 3 h", "in 2 days" for a time ahead; "5 min ago" for one behind. */
export function relative(iso: string, now = Date.now()): string {
  const m = (new Date(iso).getTime() - now) / 60_000;
  const a = Math.abs(m);
  const t = a < 1 ? "a moment" : a < 90 ? `${Math.round(a)} min` : a < 2880 ? `${Math.round(a / 60)} h` : `${Math.round(a / 1440)} days`;
  return a < 1 ? "now" : m >= 0 ? `in ${t}` : `${t} ago`;
}
