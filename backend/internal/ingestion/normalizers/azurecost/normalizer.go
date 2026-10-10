// Package azurecost normalizes the daily aggregate that the Azure provider reads through the Cost Management Query
// API (billed and amortized cost per day, service, location and currency) into FOCUS records.
//
// The API gives cost only: no quantities, no SKU, no instance size. Every line is therefore a cost line with a
// cost-based carbon estimate; nothing is measured. (A customer who also exports to Blob Storage and connects the
// export gets measured virtual machine hours: see azureexport.) The contract is the JSON below, a copy of the
// provider's struct: the two modules never import each other.
package azurecost

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

type payload struct {
	Rows []struct {
		Day       string  `json:"day"`
		Service   string  `json:"service"`
		Location  string  `json:"location"`
		Currency  string  `json:"currency"`
		Billed    float64 `json:"billed"`
		Effective float64 `json:"effective"`
	} `json:"rows"`
}

type Normalizer struct{}

// Normalize converts the aggregate into cost records.
func (Normalizer) Normalize(raw focus.RawBatch) ([]focus.Record, error) {
	var p payload
	if err := json.Unmarshal(raw.Payload, &p); err != nil {
		return nil, fmt.Errorf("azurecost: decode payload: %w", err)
	}
	out := make([]focus.Record, 0, len(p.Rows))
	for _, r := range p.Rows {
		day, err := time.Parse(time.DateOnly, r.Day)
		if err != nil {
			return nil, fmt.Errorf("azurecost: bad day %q", r.Day)
		}
		if r.Billed == 0 && r.Effective == 0 {
			continue // nothing charged: no row
		}
		service := strings.TrimSpace(r.Service)
		if service == "" {
			service = "Other"
		}
		out = append(out, focus.Record{
			Provider: "azure", Source: raw.Source, ServiceName: service, ServiceCategory: Category(service),
			RegionID: Region(r.Location), BilledCost: r.Billed, EffectiveCost: r.Effective, Currency: strings.ToUpper(strings.TrimSpace(r.Currency)),
			ChargePeriodStart: day, ChargePeriodEnd: day.AddDate(0, 0, 1),
		})
	}
	return out, nil
}

var categories = map[string]string{}

func init() {
	for category, names := range map[string][]string{
		"compute": {"Virtual Machines", "Virtual Machine Scale Sets", "Azure Kubernetes Service", "Container Instances", "Container Apps",
			"Functions", "Azure Functions", "Azure App Service", "App Service", "Azure Batch", "Batch", "Azure Spring Apps", "Azure VMware Solution",
			"Azure Databricks", "Azure Synapse Analytics", "HDInsight", "Azure Machine Learning", "Cognitive Services", "Azure OpenAI Service", "Azure AI services"},
		"storage": {"Storage", "Azure Files", "Azure NetApp Files", "Backup", "Azure Backup", "Azure Site Recovery", "Managed Disks", "Data Lake Storage"},
		"database": {"SQL Database", "Azure SQL Database", "SQL Managed Instance", "Azure Database for PostgreSQL", "Azure Database for MySQL",
			"Azure Database for MariaDB", "Azure Cosmos DB", "Azure Cache for Redis", "Redis Cache", "Azure Data Explorer", "SQL Server"},
		"networking": {"Bandwidth", "Virtual Network", "Load Balancer", "Application Gateway", "Azure Firewall", "VPN Gateway", "ExpressRoute",
			"Azure DNS", "Content Delivery Network", "Azure Front Door Service", "Azure Front Door", "Traffic Manager", "NAT Gateway", "Azure Bastion", "Network Watcher", "Azure Private Link"},
	} {
		for _, n := range names {
			categories[strings.ToLower(n)] = category
		}
	}
}

// Category maps an Azure service name to the product's reduced category set. An unknown service is "other": it keeps
// no cost-based estimate rather than a wrong one.
func Category(service string) string {
	if c, ok := categories[strings.ToLower(strings.TrimSpace(service))]; ok {
		return c
	}
	return "other"
}

var fixed = map[string]string{
	"eu west": "westeurope", "eu north": "northeurope",
	"asia pacific east": "eastasia", "asia pacific southeast": "southeastasia", "asia pacific": "southeastasia",
	"au east": "australiaeast", "au southeast": "australiasoutheast", "au central": "australiacentral",
	"in central": "centralindia", "in south": "southindia", "in west": "westindia",
	"kr central": "koreacentral", "kr south": "koreasouth", "jp east": "japaneast", "jp west": "japanwest",
	"sa north": "southafricanorth", "sa west": "southafricawest", "ae north": "uaenorth", "ae central": "uaecentral",
	"ca central": "canadacentral", "ca east": "canadaeast", "br south": "brazilsouth", "br southeast": "brazilsoutheast",
	"fr central": "francecentral", "fr south": "francesouth", "de west central": "germanywestcentral", "de north": "germanynorth",
	"ch north": "switzerlandnorth", "ch west": "switzerlandwest", "no east": "norwayeast", "no west": "norwaywest",
}

// Region maps the Cost Management location to an Azure region id ("westeurope"). The Query API names locations in
// words ("EU West", "US East 2", "UK South") while other Azure surfaces use the id, so both are accepted. Missing,
// global and zone-only values become "global": there is no grid to attribute carbon to.
func Region(v string) string {
	r := strings.ToLower(strings.TrimSpace(v))
	switch {
	case r == "", r == "global", r == "unassigned", r == "unknown", r == "n/a", r == "not applicable", strings.HasPrefix(r, "zone "):
		return "global"
	}
	r = strings.Join(strings.Fields(r), " ")
	if id, ok := fixed[r]; ok {
		return id
	}
	words := strings.Fields(r)
	if len(words) > 1 && words[0] == "us" {
		// "us east 2" -> "eastus2", "us north central" -> "northcentralus"
		digits := ""
		if last := words[len(words)-1]; strings.Trim(last, "0123456789") == "" {
			digits, words = last, words[:len(words)-1]
		}
		return strings.Join(words[1:], "") + "us" + digits
	}
	return strings.Join(words, "")
}
