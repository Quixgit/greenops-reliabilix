import { redirect } from "next/navigation";

// Cloud accounts live on the Connectors page; this address stays for old links and notifications.
export default function CloudPage() {
  redirect("/connectors#accounts");
}
