import { IconAws, IconAzure, IconCloud, IconDigitalOcean, IconGcp, IconKubernetes, type IconType } from "@/components/ui/icons";
import { cloudId } from "@/lib/clouds";

const MARK: Record<string, IconType> = { aws: IconAws, azure: IconAzure, gcp: IconGcp, digitalocean: IconDigitalOcean, kubernetes: IconKubernetes };

/** The drawn mark of a cloud; a vendor without one gets the plain cloud glyph. */
export function CloudMark({ provider, className }: { provider: string; className?: string }) {
  const Icon = MARK[cloudId(provider)] ?? IconCloud;
  return <Icon className={className} />;
}
