import latencyTable from "./latency.json";

// Cloud regions we can place on the map: city, country as spelled in world-atlas (Natural Earth), ISO-2 code for the flag,
// and the position of the data centres. Extend as regions are onboarded; an unknown region is still listed, just not drawn.
export type RegionGeo = { city: string; country: string; cc: string; lat: number; lon: number };

const g = (city: string, country: string, cc: string, lat: number, lon: number): RegionGeo => ({ city, country, cc, lat, lon });

const US = "United States of America";
export const REGION_GEO: Record<string, RegionGeo> = {
  // AWS
  "us-east-1": g("N. Virginia", US, "US", 38.9, -77.4), "us-east-2": g("Ohio", US, "US", 40.0, -83.0),
  "us-west-1": g("N. California", US, "US", 37.4, -122.0), "us-west-2": g("Oregon", US, "US", 45.8, -119.7),
  "ca-central-1": g("Montréal", "Canada", "CA", 45.5, -73.6), "ca-west-1": g("Calgary", "Canada", "CA", 51.0, -114.1),
  "eu-west-1": g("Ireland", "Ireland", "IE", 53.3, -6.3), "eu-west-2": g("London", "United Kingdom", "GB", 51.5, -0.1),
  "eu-west-3": g("Paris", "France", "FR", 48.9, 2.3), "eu-central-1": g("Frankfurt", "Germany", "DE", 50.1, 8.7),
  "eu-central-2": g("Zurich", "Switzerland", "CH", 47.4, 8.5), "eu-north-1": g("Stockholm", "Sweden", "SE", 59.3, 18.1),
  "eu-south-1": g("Milan", "Italy", "IT", 45.5, 9.2), "eu-south-2": g("Aragón", "Spain", "ES", 41.6, -0.9),
  "ap-northeast-1": g("Tokyo", "Japan", "JP", 35.7, 139.7), "ap-northeast-2": g("Seoul", "South Korea", "KR", 37.6, 127.0),
  "ap-northeast-3": g("Osaka", "Japan", "JP", 34.7, 135.5), "ap-southeast-1": g("Singapore", "Singapore", "SG", 1.35, 103.8),
  "ap-southeast-2": g("Sydney", "Australia", "AU", -33.9, 151.2), "ap-southeast-4": g("Melbourne", "Australia", "AU", -37.8, 145.0),
  "ap-south-1": g("Mumbai", "India", "IN", 19.1, 72.9), "ap-east-1": g("Hong Kong", "Hong Kong", "HK", 22.3, 114.2),
  "sa-east-1": g("São Paulo", "Brazil", "BR", -23.5, -46.6), "il-central-1": g("Tel Aviv", "Israel", "IL", 32.1, 34.8),
  "af-south-1": g("Cape Town", "South Africa", "ZA", -33.9, 18.4), "me-south-1": g("Bahrain", "Bahrain", "BH", 26.1, 50.5),
  // Google Cloud
  "us-central1": g("Iowa", US, "US", 41.3, -95.9), "us-east1": g("S. Carolina", US, "US", 33.2, -80.0),
  "us-east4": g("N. Virginia", US, "US", 39.0, -77.5), "us-east5": g("Columbus", US, "US", 40.0, -83.0),
  "us-south1": g("Dallas", US, "US", 32.8, -96.8), "us-west1": g("Oregon", US, "US", 45.6, -121.2),
  "us-west2": g("Los Angeles", US, "US", 34.1, -118.2), "us-west3": g("Salt Lake City", US, "US", 40.8, -111.9),
  "northamerica-northeast1": g("Montréal", "Canada", "CA", 45.5, -73.6), "northamerica-northeast2": g("Toronto", "Canada", "CA", 43.7, -79.4),
  "southamerica-east1": g("São Paulo", "Brazil", "BR", -23.5, -46.6), "southamerica-west1": g("Santiago", "Chile", "CL", -33.4, -70.7),
  "europe-north1": g("Hamina", "Finland", "FI", 60.6, 27.2), "europe-west1": g("St. Ghislain", "Belgium", "BE", 50.5, 3.8),
  "europe-west2": g("London", "United Kingdom", "GB", 51.5, -0.1), "europe-west3": g("Frankfurt", "Germany", "DE", 50.1, 8.7),
  "europe-west4": g("Eemshaven", "Netherlands", "NL", 53.4, 6.8), "europe-west6": g("Zurich", "Switzerland", "CH", 47.4, 8.5),
  "europe-west8": g("Milan", "Italy", "IT", 45.5, 9.2), "europe-west9": g("Paris", "France", "FR", 48.9, 2.3),
  "europe-west12": g("Turin", "Italy", "IT", 45.1, 7.7), "europe-southwest1": g("Madrid", "Spain", "ES", 40.4, -3.7),
  "europe-central2": g("Warsaw", "Poland", "PL", 52.2, 21.0),
  "asia-east1": g("Changhua", "Taiwan", "TW", 24.1, 120.5), "asia-east2": g("Hong Kong", "Hong Kong", "HK", 22.3, 114.2),
  "asia-northeast1": g("Tokyo", "Japan", "JP", 35.7, 139.7), "asia-northeast2": g("Osaka", "Japan", "JP", 34.7, 135.5),
  "asia-northeast3": g("Seoul", "South Korea", "KR", 37.6, 127.0), "asia-south1": g("Mumbai", "India", "IN", 19.1, 72.9),
  "asia-south2": g("Delhi", "India", "IN", 28.6, 77.2), "asia-southeast1": g("Singapore", "Singapore", "SG", 1.35, 103.8),
  "australia-southeast1": g("Sydney", "Australia", "AU", -33.9, 151.2), "australia-southeast2": g("Melbourne", "Australia", "AU", -37.8, 145.0),
  "me-west1": g("Tel Aviv", "Israel", "IL", 32.1, 34.8), "me-central1": g("Doha", "Qatar", "QA", 25.3, 51.5),
  "africa-south1": g("Johannesburg", "South Africa", "ZA", -26.2, 28.0),
  // Microsoft Azure
  "eastus": g("Virginia", US, "US", 37.4, -79.5), "eastus2": g("Virginia", US, "US", 36.7, -78.4), "eastus3": g("Atlanta", US, "US", 33.7, -84.4),
  "centralus": g("Iowa", US, "US", 41.6, -93.6), "northcentralus": g("Illinois", US, "US", 41.9, -87.6),
  "southcentralus": g("Texas", US, "US", 29.4, -98.5), "westus": g("California", US, "US", 37.8, -122.4),
  "westus2": g("Washington", US, "US", 47.2, -119.9), "westus3": g("Arizona", US, "US", 33.4, -112.1),
  "westcentralus": g("Wyoming", US, "US", 41.1, -104.8),
  "canadacentral": g("Toronto", "Canada", "CA", 43.7, -79.4), "canadaeast": g("Québec", "Canada", "CA", 46.8, -71.2),
  "brazilsouth": g("São Paulo", "Brazil", "BR", -23.5, -46.6), "brazilsoutheast": g("Rio de Janeiro", "Brazil", "BR", -22.9, -43.2),
  "northeurope": g("Ireland", "Ireland", "IE", 53.3, -6.3), "westeurope": g("Netherlands", "Netherlands", "NL", 52.4, 4.9),
  "uksouth": g("London", "United Kingdom", "GB", 51.5, -0.1), "ukwest": g("Cardiff", "United Kingdom", "GB", 51.5, -3.2),
  "francecentral": g("Paris", "France", "FR", 48.9, 2.3), "francesouth": g("Marseille", "France", "FR", 43.3, 5.4),
  "germanywestcentral": g("Frankfurt", "Germany", "DE", 50.1, 8.7), "germanynorth": g("Berlin", "Germany", "DE", 52.5, 13.4),
  "switzerlandnorth": g("Zurich", "Switzerland", "CH", 47.4, 8.5), "switzerlandwest": g("Geneva", "Switzerland", "CH", 46.2, 6.1),
  "norwayeast": g("Oslo", "Norway", "NO", 59.9, 10.8), "norwaywest": g("Stavanger", "Norway", "NO", 58.97, 5.7),
  "swedencentral": g("Gävle", "Sweden", "SE", 60.7, 17.1), "polandcentral": g("Warsaw", "Poland", "PL", 52.2, 21.0),
  "italynorth": g("Milan", "Italy", "IT", 45.5, 9.2), "spaincentral": g("Madrid", "Spain", "ES", 40.4, -3.7),
  "austriaeast": g("Vienna", "Austria", "AT", 48.2, 16.4), "denmarkeast": g("Copenhagen", "Denmark", "DK", 55.7, 12.6),
  "belgiumcentral": g("Brussels", "Belgium", "BE", 50.8, 4.4),
  "japaneast": g("Tokyo", "Japan", "JP", 35.7, 139.7), "japanwest": g("Osaka", "Japan", "JP", 34.7, 135.5),
  "koreacentral": g("Seoul", "South Korea", "KR", 37.6, 127.0), "koreasouth": g("Busan", "South Korea", "KR", 35.2, 129.1),
  "southeastasia": g("Singapore", "Singapore", "SG", 1.35, 103.8), "eastasia": g("Hong Kong", "Hong Kong", "HK", 22.3, 114.2),
  "australiaeast": g("Sydney", "Australia", "AU", -33.9, 151.2), "australiasoutheast": g("Melbourne", "Australia", "AU", -37.8, 145.0),
  "australiacentral": g("Canberra", "Australia", "AU", -35.3, 149.1),
  "centralindia": g("Pune", "India", "IN", 18.5, 73.9), "southindia": g("Chennai", "India", "IN", 13.1, 80.3),
  "westindia": g("Mumbai", "India", "IN", 19.1, 72.9), "indonesiacentral": g("Jakarta", "Indonesia", "ID", -6.2, 106.8),
  "malaysiawest": g("Kuala Lumpur", "Malaysia", "MY", 3.1, 101.7), "newzealandnorth": g("Auckland", "New Zealand", "NZ", -36.9, 174.8),
  "taiwannorth": g("Taipei", "Taiwan", "TW", 25.0, 121.5),
  "uaenorth": g("Dubai", "United Arab Emirates", "AE", 25.3, 55.3), "uaecentral": g("Abu Dhabi", "United Arab Emirates", "AE", 24.5, 54.4),
  "qatarcentral": g("Doha", "Qatar", "QA", 25.3, 51.5), "israelcentral": g("Tel Aviv", "Israel", "IL", 32.1, 34.8),
  "southafricanorth": g("Johannesburg", "South Africa", "ZA", -26.2, 28.0), "southafricawest": g("Cape Town", "South Africa", "ZA", -33.9, 18.4),
  "chilecentral": g("Santiago", "Chile", "CL", -33.4, -70.7),
};

export const REGION_COUNTRY: Record<string, string> = Object.fromEntries(Object.entries(REGION_GEO).map(([k, v]) => [k, v.country]));

export const REGION_LABEL: Record<string, string> = {
  ...Object.fromEntries(Object.entries(REGION_GEO).map(([k, v]) => [k, `${v.city}, ${v.country}`])),
  "us-east-1": "US East (N. Virginia)",
  "us-west-2": "US West (Oregon)",
  "eu-west-1": "Europe (Ireland)",
  "eu-central-1": "Europe (Frankfurt)",
  "eu-north-1": "Europe (Stockholm)",
};

/** Cloud provider by the shape of the region id: `eu-central-1` is AWS, `europe-west6` is Google Cloud, `westeurope` is Azure. */
export function regionProvider(id: string): "AWS" | "GCP" | "Azure" | "Other" {
  if (/^[a-z]{2}(-[a-z]+)+-\d$/.test(id)) return "AWS";
  if (/^[a-z]+(-[a-z]+)?\d+$/.test(id) && id.includes("-")) return "GCP";
  if (/^[a-z]+\d*$/.test(id)) return "Azure";
  return "Other";
}

/** Great-circle distance between two known regions in km, or null when either is not on the map. */
export function distanceKm(a: string, b: string): number | null {
  const p = REGION_GEO[a], q = REGION_GEO[b];
  if (!p || !q) return null;
  const rad = Math.PI / 180, dLat = (q.lat - p.lat) * rad, dLon = (q.lon - p.lon) * rad;
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(p.lat * rad) * Math.cos(q.lat * rad) * Math.sin(dLon / 2) ** 2;
  return 2 * 6371 * Math.asin(Math.sqrt(h));
}

export type LatencyBasis = "measured_azure" | "measured_aws" | "estimated";
export type Latency = { ms: number; km: number | null; basis: LatencyBasis; errorPct: number };

type Matrix = Record<string, Record<string, number>>;
const AZURE = latencyTable.azure.ms as Matrix, AWS = latencyTable.aws.ms as Matrix, FIT = latencyTable.fit;

function pick(m: Matrix, a: string, b: string): number | null {
  const x = m[a]?.[b], y = m[b]?.[a];
  if (x != null && y != null) return Math.max(x, y);
  return x ?? y ?? null;
}

/**
 * Round trip between two regions, as the backend (`geo.Between`) computes it: a figure published by Microsoft (Azure pairs) or
 * measured by CloudPing (AWS pairs, median over a year) where one exists, otherwise an estimate from the distance with a line fitted
 * to the measured pairs (`errorPct`: 80% of them are within it). Null when the pair is unknown and a region is not on the map.
 */
export function latencyBetween(a: string, b: string): Latency | null {
  const km = distanceKm(a, b);
  if (a !== b) {
    const az = pick(AZURE, a, b);
    if (az !== null) return { ms: Math.max(1, az), km, basis: "measured_azure", errorPct: 0 };
    const aws = pick(AWS, a, b);
    if (aws !== null) return { ms: Math.max(1, aws), km, basis: "measured_aws", errorPct: 0 };
  }
  if (km === null) return null;
  return { ms: Math.max(1, Math.round(FIT.intercept_ms + FIT.per_km_ms * km)), km, basis: "estimated", errorPct: FIT.p80_error_pct };
}

/** Flag emoji from an ISO-2 code (regional indicator letters). */
export const flag = (cc: string) => String.fromCodePoint(...[...cc.toUpperCase()].map((c) => 127397 + c.charCodeAt(0)));

/** A region as a person reads it: "Toronto, Canada" when the platform knows the place, the provider's own id otherwise. */
export function regionName(id: string): string {
  const g = REGION_GEO[id];
  return g ? `${g.city}, ${g.country}` : id;
}
