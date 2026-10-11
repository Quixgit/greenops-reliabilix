// Package geo knows where cloud regions are, so a move from one to another can be told in kilometres and milliseconds.
// The table is the one the console draws its map from (frontend/lib/regions.ts); an unknown region is simply not placed.
package geo

import "math"

// Place is where a region's data centres are.
type Place struct {
	City, Country, CC string
	Lat, Lon          float64
}

// Regions by the provider's own id.
var Regions = map[string]Place{
	"us-east-1":               {"N. Virginia", "United States of America", "US", 38.9, -77.4},
	"us-east-2":               {"Ohio", "United States of America", "US", 40.0, -83.0},
	"us-west-1":               {"N. California", "United States of America", "US", 37.4, -122.0},
	"us-west-2":               {"Oregon", "United States of America", "US", 45.8, -119.7},
	"ca-central-1":            {"Montréal", "Canada", "CA", 45.5, -73.6},
	"ca-west-1":               {"Calgary", "Canada", "CA", 51.0, -114.1},
	"eu-west-1":               {"Ireland", "Ireland", "IE", 53.3, -6.3},
	"eu-west-2":               {"London", "United Kingdom", "GB", 51.5, -0.1},
	"eu-west-3":               {"Paris", "France", "FR", 48.9, 2.3},
	"eu-central-1":            {"Frankfurt", "Germany", "DE", 50.1, 8.7},
	"eu-central-2":            {"Zurich", "Switzerland", "CH", 47.4, 8.5},
	"eu-north-1":              {"Stockholm", "Sweden", "SE", 59.3, 18.1},
	"eu-south-1":              {"Milan", "Italy", "IT", 45.5, 9.2},
	"eu-south-2":              {"Aragón", "Spain", "ES", 41.6, -0.9},
	"ap-northeast-1":          {"Tokyo", "Japan", "JP", 35.7, 139.7},
	"ap-northeast-2":          {"Seoul", "South Korea", "KR", 37.6, 127.0},
	"ap-northeast-3":          {"Osaka", "Japan", "JP", 34.7, 135.5},
	"ap-southeast-1":          {"Singapore", "Singapore", "SG", 1.35, 103.8},
	"ap-southeast-2":          {"Sydney", "Australia", "AU", -33.9, 151.2},
	"ap-southeast-4":          {"Melbourne", "Australia", "AU", -37.8, 145.0},
	"ap-south-1":              {"Mumbai", "India", "IN", 19.1, 72.9},
	"ap-east-1":               {"Hong Kong", "Hong Kong", "HK", 22.3, 114.2},
	"sa-east-1":               {"São Paulo", "Brazil", "BR", -23.5, -46.6},
	"il-central-1":            {"Tel Aviv", "Israel", "IL", 32.1, 34.8},
	"af-south-1":              {"Cape Town", "South Africa", "ZA", -33.9, 18.4},
	"me-south-1":              {"Bahrain", "Bahrain", "BH", 26.1, 50.5},
	"us-central1":             {"Iowa", "United States of America", "US", 41.3, -95.9},
	"us-east1":                {"S. Carolina", "United States of America", "US", 33.2, -80.0},
	"us-east4":                {"N. Virginia", "United States of America", "US", 39.0, -77.5},
	"us-east5":                {"Columbus", "United States of America", "US", 40.0, -83.0},
	"us-south1":               {"Dallas", "United States of America", "US", 32.8, -96.8},
	"us-west1":                {"Oregon", "United States of America", "US", 45.6, -121.2},
	"us-west2":                {"Los Angeles", "United States of America", "US", 34.1, -118.2},
	"us-west3":                {"Salt Lake City", "United States of America", "US", 40.8, -111.9},
	"northamerica-northeast1": {"Montréal", "Canada", "CA", 45.5, -73.6},
	"northamerica-northeast2": {"Toronto", "Canada", "CA", 43.7, -79.4},
	"southamerica-east1":      {"São Paulo", "Brazil", "BR", -23.5, -46.6},
	"southamerica-west1":      {"Santiago", "Chile", "CL", -33.4, -70.7},
	"europe-north1":           {"Hamina", "Finland", "FI", 60.6, 27.2},
	"europe-west1":            {"St. Ghislain", "Belgium", "BE", 50.5, 3.8},
	"europe-west2":            {"London", "United Kingdom", "GB", 51.5, -0.1},
	"europe-west3":            {"Frankfurt", "Germany", "DE", 50.1, 8.7},
	"europe-west4":            {"Eemshaven", "Netherlands", "NL", 53.4, 6.8},
	"europe-west6":            {"Zurich", "Switzerland", "CH", 47.4, 8.5},
	"europe-west8":            {"Milan", "Italy", "IT", 45.5, 9.2},
	"europe-west9":            {"Paris", "France", "FR", 48.9, 2.3},
	"europe-west12":           {"Turin", "Italy", "IT", 45.1, 7.7},
	"europe-southwest1":       {"Madrid", "Spain", "ES", 40.4, -3.7},
	"europe-central2":         {"Warsaw", "Poland", "PL", 52.2, 21.0},
	"asia-east1":              {"Changhua", "Taiwan", "TW", 24.1, 120.5},
	"asia-east2":              {"Hong Kong", "Hong Kong", "HK", 22.3, 114.2},
	"asia-northeast1":         {"Tokyo", "Japan", "JP", 35.7, 139.7},
	"asia-northeast2":         {"Osaka", "Japan", "JP", 34.7, 135.5},
	"asia-northeast3":         {"Seoul", "South Korea", "KR", 37.6, 127.0},
	"asia-south1":             {"Mumbai", "India", "IN", 19.1, 72.9},
	"asia-south2":             {"Delhi", "India", "IN", 28.6, 77.2},
	"asia-southeast1":         {"Singapore", "Singapore", "SG", 1.35, 103.8},
	"australia-southeast1":    {"Sydney", "Australia", "AU", -33.9, 151.2},
	"australia-southeast2":    {"Melbourne", "Australia", "AU", -37.8, 145.0},
	"me-west1":                {"Tel Aviv", "Israel", "IL", 32.1, 34.8},
	"me-central1":             {"Doha", "Qatar", "QA", 25.3, 51.5},
	"africa-south1":           {"Johannesburg", "South Africa", "ZA", -26.2, 28.0},
	"eastus":                  {"Virginia", "United States of America", "US", 37.4, -79.5},
	"eastus2":                 {"Virginia", "United States of America", "US", 36.7, -78.4},
	"eastus3":                 {"Atlanta", "United States of America", "US", 33.7, -84.4},
	"centralus":               {"Iowa", "United States of America", "US", 41.6, -93.6},
	"northcentralus":          {"Illinois", "United States of America", "US", 41.9, -87.6},
	"southcentralus":          {"Texas", "United States of America", "US", 29.4, -98.5},
	"westus":                  {"California", "United States of America", "US", 37.8, -122.4},
	"westus2":                 {"Washington", "United States of America", "US", 47.2, -119.9},
	"westus3":                 {"Arizona", "United States of America", "US", 33.4, -112.1},
	"westcentralus":           {"Wyoming", "United States of America", "US", 41.1, -104.8},
	"canadacentral":           {"Toronto", "Canada", "CA", 43.7, -79.4},
	"canadaeast":              {"Québec", "Canada", "CA", 46.8, -71.2},
	"brazilsouth":             {"São Paulo", "Brazil", "BR", -23.5, -46.6},
	"brazilsoutheast":         {"Rio de Janeiro", "Brazil", "BR", -22.9, -43.2},
	"northeurope":             {"Ireland", "Ireland", "IE", 53.3, -6.3},
	"westeurope":              {"Netherlands", "Netherlands", "NL", 52.4, 4.9},
	"uksouth":                 {"London", "United Kingdom", "GB", 51.5, -0.1},
	"ukwest":                  {"Cardiff", "United Kingdom", "GB", 51.5, -3.2},
	"francecentral":           {"Paris", "France", "FR", 48.9, 2.3},
	"francesouth":             {"Marseille", "France", "FR", 43.3, 5.4},
	"germanywestcentral":      {"Frankfurt", "Germany", "DE", 50.1, 8.7},
	"germanynorth":            {"Berlin", "Germany", "DE", 52.5, 13.4},
	"switzerlandnorth":        {"Zurich", "Switzerland", "CH", 47.4, 8.5},
	"switzerlandwest":         {"Geneva", "Switzerland", "CH", 46.2, 6.1},
	"norwayeast":              {"Oslo", "Norway", "NO", 59.9, 10.8},
	"norwaywest":              {"Stavanger", "Norway", "NO", 58.97, 5.7},
	"swedencentral":           {"Gävle", "Sweden", "SE", 60.7, 17.1},
	"polandcentral":           {"Warsaw", "Poland", "PL", 52.2, 21.0},
	"italynorth":              {"Milan", "Italy", "IT", 45.5, 9.2},
	"spaincentral":            {"Madrid", "Spain", "ES", 40.4, -3.7},
	"austriaeast":             {"Vienna", "Austria", "AT", 48.2, 16.4},
	"denmarkeast":             {"Copenhagen", "Denmark", "DK", 55.7, 12.6},
	"belgiumcentral":          {"Brussels", "Belgium", "BE", 50.8, 4.4},
	"japaneast":               {"Tokyo", "Japan", "JP", 35.7, 139.7},
	"japanwest":               {"Osaka", "Japan", "JP", 34.7, 135.5},
	"koreacentral":            {"Seoul", "South Korea", "KR", 37.6, 127.0},
	"koreasouth":              {"Busan", "South Korea", "KR", 35.2, 129.1},
	"southeastasia":           {"Singapore", "Singapore", "SG", 1.35, 103.8},
	"eastasia":                {"Hong Kong", "Hong Kong", "HK", 22.3, 114.2},
	"australiaeast":           {"Sydney", "Australia", "AU", -33.9, 151.2},
	"australiasoutheast":      {"Melbourne", "Australia", "AU", -37.8, 145.0},
	"australiacentral":        {"Canberra", "Australia", "AU", -35.3, 149.1},
	"centralindia":            {"Pune", "India", "IN", 18.5, 73.9},
	"southindia":              {"Chennai", "India", "IN", 13.1, 80.3},
	"westindia":               {"Mumbai", "India", "IN", 19.1, 72.9},
	"indonesiacentral":        {"Jakarta", "Indonesia", "ID", -6.2, 106.8},
	"malaysiawest":            {"Kuala Lumpur", "Malaysia", "MY", 3.1, 101.7},
	"newzealandnorth":         {"Auckland", "New Zealand", "NZ", -36.9, 174.8},
	"taiwannorth":             {"Taipei", "Taiwan", "TW", 25.0, 121.5},
	"uaenorth":                {"Dubai", "United Arab Emirates", "AE", 25.3, 55.3},
	"uaecentral":              {"Abu Dhabi", "United Arab Emirates", "AE", 24.5, 54.4},
	"qatarcentral":            {"Doha", "Qatar", "QA", 25.3, 51.5},
	"israelcentral":           {"Tel Aviv", "Israel", "IL", 32.1, 34.8},
	"southafricanorth":        {"Johannesburg", "South Africa", "ZA", -26.2, 28.0},
	"southafricawest":         {"Cape Town", "South Africa", "ZA", -33.9, 18.4},
	"chilecentral":            {"Santiago", "Chile", "CL", -33.4, -70.7},
}

// DistanceKm is the great-circle distance between two known regions, and false when either is not on the map.
func DistanceKm(a, b string) (float64, bool) {
	p, ok1 := Regions[a]
	q, ok2 := Regions[b]
	if !ok1 || !ok2 {
		return 0, false
	}
	const rad = math.Pi / 180
	dLat, dLon := (q.Lat-p.Lat)*rad, (q.Lon-p.Lon)*rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(p.Lat*rad)*math.Cos(q.Lat*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * 6371 * math.Asin(math.Sqrt(h)), true
}

// RTTms is the round-trip time between two known regions in milliseconds, measured where a measurement exists and estimated from
// the distance otherwise (see Between, which also says which). The second value is false when the pair cannot be told.
func RTTms(a, b string) (int, bool) {
	l, ok := Between(a, b)
	return l.Ms, ok
}
