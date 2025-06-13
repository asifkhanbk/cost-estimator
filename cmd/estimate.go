package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// --- Pricing mapping ---

type PricingDefinition struct {
	ServiceName    string
	SKUKeys        []string
	RegionKey      string
	UsageExtractor func(map[string]interface{}) (float64, string)
}

var ResourceTypePricingMap = map[string]PricingDefinition{
	// Add all your Azure resources, example subset:
	"azurerm_kubernetes_cluster_node_pool": {
		ServiceName: "Virtual Machines", SKUKeys: []string{"vm_size", "sku"}, RegionKey: "location",
	},
	"azurerm_kubernetes_cluster": {
		ServiceName: "Kubernetes Service", SKUKeys: []string{"sku_tier"}, RegionKey: "location",
	},
	"azurerm_linux_virtual_machine": {
		ServiceName: "Virtual Machines", SKUKeys: []string{"size", "vm_size", "sku"}, RegionKey: "location",
	},
	"azurerm_windows_virtual_machine": {
		ServiceName: "Virtual Machines", SKUKeys: []string{"size", "vm_size", "sku"}, RegionKey: "location",
	},
	"azurerm_managed_disk": {
		ServiceName: "Storage", SKUKeys: []string{"sku_name"}, RegionKey: "location",
		UsageExtractor: func(vals map[string]interface{}) (float64, string) {
			sizeGB := 0.0
			if v, ok := vals["disk_size_gb"]; ok {
				fmt.Sscanf(fmt.Sprintf("%v", v), "%f", &sizeGB)
			}
			return sizeGB, fmt.Sprintf("%.0f GB", sizeGB)
		},
	},
	"azurerm_storage_account": {
		ServiceName: "Storage", SKUKeys: []string{"account_tier", "sku_name"}, RegionKey: "location",
	},
	"azurerm_private_endpoint": {
		ServiceName: "Private Link", SKUKeys: []string{}, RegionKey: "location",
	},
	"azurerm_public_ip": {
		ServiceName: "IP Addresses", SKUKeys: []string{"sku"}, RegionKey: "location",
	},
	"azurerm_virtual_network": {
		ServiceName: "Virtual Network", SKUKeys: []string{}, RegionKey: "location",
	},
	"azurerm_subnet": {
		ServiceName: "Virtual Network", SKUKeys: []string{}, RegionKey: "location",
	},
	// ... extend as you wish!
	"azurerm_key_vault": {
		ServiceName: "Key Vault", SKUKeys: []string{"sku_name"}, RegionKey: "location",
		UsageExtractor: func(vals map[string]interface{}) (float64, string) {
			// Simulate 20,000 ops for demo
			simulatedUsage := 20000.0
			return simulatedUsage, fmt.Sprintf("%.0f operations", simulatedUsage)
		},
	},
}

// --- Resource extraction ---

type Resource struct {
	Type        string
	Name        string
	Region      string
	Values      map[string]interface{}
	Address     string
	RefResource string // If this resource references another resource for region
}

var estimateCmd = &cobra.Command{
	Use:   "estimate",
	Short: "Estimate costs from a Terraform JSON plan",
	Run:   runEstimate,
}

func init() {
	rootCmd.AddCommand(estimateCmd)
}

func runEstimate(cmd *cobra.Command, args []string) {
	if planFile == "" {
		fmt.Println("❌ --plan is required")
		return
	}
	data, err := os.ReadFile(planFile)
	if err != nil {
		fmt.Printf("❌ Failed to read plan: %v\n", err)
		return
	}
	var plan map[string]interface{}
	if err := json.Unmarshal(data, &plan); err != nil {
		fmt.Printf("❌ Failed to parse JSON: %v\n", err)
		return
	}
	variables := extractVariables(plan)
	addressResourceMap := map[string]*Resource{} // map of resource address to Resource (for ref lookup)
	rootMod, ok := plan["planned_values"].(map[string]interface{})["root_module"].(map[string]interface{})
	if !ok {
		fmt.Println("⚠️ No root_module in plan")
		return
	}
	resources := extractResources(rootMod, addressResourceMap, "")

	// For nodepool region inheritance: build clusterName -> region map
	clusterRegions := buildClusterRegionMap(resources)

	// Render
	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"Type", "Name", "Region", "SKU / Detail", "Unit", "Usage", "Unit Cost", "Monthly Cost"})
	var total float64

	for _, r := range resources {
		def, found := ResourceTypePricingMap[r.Type]
		if !found {
			def = PricingDefinition{
				ServiceName:    guessServiceName(r.Type),
				SKUKeys:        []string{"sku", "sku_name", "size"},
				RegionKey:      "location",
				UsageExtractor: nil,
			}
		}
		region := extractStringWithVars(r.Values, def.RegionKey, variables, addressResourceMap)
		if region == "" {
			region = r.Region
		}
		// Special: for AKS node pools, if no region, inherit from parent cluster (by cluster_name)
		if r.Type == "azurerm_kubernetes_cluster_node_pool" && region == "" {
			clusterName := extractStringWithVars(r.Values, "cluster_name", variables, addressResourceMap)
			region = clusterRegions[clusterName]
		}
		// --- SKU resolution ---
		sku := ""
		for _, key := range def.SKUKeys {
			sku = extractStringWithVars(r.Values, key, variables, addressResourceMap)
			if sku != "" {
				break
			}
		}
		quantity := 1.0
		quantityDesc := "-"
		if def.UsageExtractor != nil {
			quantity, quantityDesc = def.UsageExtractor(r.Values)
			if quantity == 0 {
				quantity = 1
			}
		}
		unitCost, unit, foundPrice := tryAllPricing(def.ServiceName, region, sku)
		monthlyCost := 0.0
		usageText := quantityDesc

		// --- Fallback for private endpoint pricing if API fails ---
		if r.Type == "azurerm_private_endpoint" && !foundPrice {
			unitCost = 0.01
			unit = "1 Hour"
			monthlyCost = unitCost * 730 * quantity
			usageText = fmt.Sprintf("%.0f x 730 hours", quantity)
			foundPrice = true
		}

		if foundPrice {
			if strings.Contains(strings.ToLower(unit), "hour") {
				monthlyCost = unitCost * 730 * quantity
				if usageText == "-" {
					usageText = fmt.Sprintf("%.0f x 730 hours", quantity)
				}
			} else if strings.Contains(strings.ToLower(unit), "gb") && quantity > 0 {
				monthlyCost = unitCost * quantity
			} else if strings.Contains(strings.ToLower(unit), "operation") && quantity > 0 {
				monthlyCost = unitCost * quantity
			} else {
				monthlyCost = unitCost * quantity
			}
			total += monthlyCost
		}

		table.Append([]string{
			r.Type, r.Name, region, sku, unit, usageText, fmt.Sprintf("%.6f", unitCost), fmt.Sprintf("%.2f", monthlyCost),
		})
	}
	table.Render()
	fmt.Printf("\n💰 Total Estimated Monthly Cost: $%.2f\n", total)
}

// --- Variable and reference resolution ---

func extractVariables(plan map[string]interface{}) map[string]string {
	vars := make(map[string]string)
	if vmap, ok := plan["variables"].(map[string]interface{}); ok {
		for k, v := range vmap {
			if valObj, ok := v.(map[string]interface{}); ok {
				if val, ok := valObj["value"]; ok {
					vars[k] = fmt.Sprintf("%v", val)
				}
			}
		}
	}
	if vmap, ok := plan["variable_values"].(map[string]interface{}); ok {
		for k, v := range vmap {
			vars[k] = fmt.Sprintf("%v", v)
		}
	}
	return vars
}

// Tries to extract a string value, resolving variable or resource field references (deep).
func extractStringWithVars(m map[string]interface{}, key string, variables map[string]string, resourceMap map[string]*Resource) string {
	if v, ok := m[key]; ok && v != nil {
		switch vv := v.(type) {
		case string:
			return vv
		case map[string]interface{}:
			if refs, ok := vv["references"].([]interface{}); ok && len(refs) > 0 {
				for _, ref := range refs {
					if refstr, ok := ref.(string); ok {
						// Terraform variable
						if strings.HasPrefix(refstr, "var.") {
							varkey := strings.TrimPrefix(refstr, "var.")
							if val, found := variables[varkey]; found {
								return val
							}
						}
						// Terraform resource reference, e.g., azurerm_kubernetes_cluster.this.location
						if parts := strings.Split(refstr, "."); len(parts) >= 3 {
							addr := strings.Join(parts[:3], ".")
							if res, ok := resourceMap[addr]; ok {
								field := ""
								if len(parts) > 3 {
									field = parts[3]
								} else if key != "" {
									field = key
								}
								if field != "" {
									if val, ok := res.Values[field]; ok && val != nil {
										// Recursively resolve!
										return extractStringWithVars(res.Values, field, variables, resourceMap)
									}
								}
							}
						}
					}
				}
			}
			// Constant value
			if cval, ok := vv["constant_value"]; ok {
				return fmt.Sprintf("%v", cval)
			}
		}
	}
	return ""
}

// --- Resource extraction with address map building ---

func extractResources(mod map[string]interface{}, resourceMap map[string]*Resource, modPrefix string) []Resource {
	var out []Resource
	if arr, ok := mod["resources"].([]interface{}); ok {
		for _, x := range arr {
			m := x.(map[string]interface{})
			vals := m["values"].(map[string]interface{})
			r := Resource{
				Type:    toString(m["type"]),
				Name:    toString(m["name"]),
				Region:  "", // Will be resolved
				Values:  vals,
				Address: toString(m["address"]),
			}
			resourceMap[r.Address] = &r
			out = append(out, r)
		}
	}
	if children, ok := mod["child_modules"].([]interface{}); ok {
		for _, c := range children {
			out = append(out, extractResources(c.(map[string]interface{}), resourceMap, modPrefix)...)
		}
	}
	return out
}

// --- AKS clusterName -> region map (for nodepools) ---

func buildClusterRegionMap(resources []Resource) map[string]string {
	clusterRegions := map[string]string{}
	for _, r := range resources {
		if r.Type == "azurerm_kubernetes_cluster" {
			name := extractStringWithVars(r.Values, "name", map[string]string{}, map[string]*Resource{})
			region := extractStringWithVars(r.Values, "location", map[string]string{}, map[string]*Resource{})
			if name != "" && region != "" {
				clusterRegions[name] = region
			}
		}
	}
	return clusterRegions
}

// --- Pricing API ---

func tryAllPricing(service, region, sku string) (float64, string, bool) {
	if service == "" {
		return 0, "", false
	}
	client := &http.Client{Timeout: 10 * time.Second}
	base := "https://prices.azure.com/api/retail/prices"

	var filters []string
	if region != "" && sku != "" {
		filters = append(filters, fmt.Sprintf("serviceName eq '%s' and armRegionName eq '%s' and (skuName eq '%s' or armSkuName eq '%s')", service, region, sku, sku))
	}
	if region != "" {
		filters = append(filters, fmt.Sprintf("serviceName eq '%s' and armRegionName eq '%s'", service, region))
	}
	filters = append(filters, fmt.Sprintf("serviceName eq '%s'", service))

	for _, filter := range filters {
		urlStr := base + "?$filter=" + url.QueryEscape(filter)
		for page := 0; urlStr != ""; page++ {
			resp, err := client.Get(urlStr)
			if err != nil {
				break
			}
			defer resp.Body.Close()

			var out struct {
				Items        []struct {
					RetailPrice   float64 `json:"retailPrice"`
					UnitOfMeasure string  `json:"unitOfMeasure"`
					MeterName     string  `json:"meterName"`
					ArmSkuName    string  `json:"armSkuName"`
					SkuName       string  `json:"skuName"`
					ArmRegionName string  `json:"armRegionName"`
				} `json:"Items"`
				NextPageLink string `json:"NextPageLink"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				break
			}

			// Private Endpoint must match "Private Endpoint" meterName exactly
			if service == "Private Link" {
				for _, item := range out.Items {
					if strings.EqualFold(item.MeterName, "Private Endpoint") &&
						strings.EqualFold(item.ArmRegionName, region) &&
						strings.Contains(strings.ToLower(item.UnitOfMeasure), "hour") &&
						item.RetailPrice > 0 {
						return item.RetailPrice, item.UnitOfMeasure, true
					}
				}
				urlStr = out.NextPageLink
				continue
			}

			for _, item := range out.Items {
				if item.RetailPrice > 0 {
					if sku != "" && (item.ArmSkuName == sku || item.SkuName == sku || strings.Contains(item.MeterName, sku)) {
						return item.RetailPrice, item.UnitOfMeasure, true
					}
					if strings.Contains(strings.ToLower(item.UnitOfMeasure), "operation") || strings.Contains(strings.ToLower(item.MeterName), "operation") {
						return item.RetailPrice, item.UnitOfMeasure, true
					}
					if sku == "" {
						return item.RetailPrice, item.UnitOfMeasure, true
					}
				}
			}
			urlStr = out.NextPageLink
		}
	}
	return 0, "", false
}

func guessServiceName(resType string) string {
	switch {
	case strings.Contains(resType, "linux_virtual_machine"), strings.Contains(resType, "windows_virtual_machine"), strings.Contains(resType, "node_pool"):
		return "Virtual Machines"
	case strings.Contains(resType, "kubernetes_cluster"):
		return "Kubernetes Service"
	case strings.Contains(resType, "storage"), strings.Contains(resType, "disk"):
		return "Storage"
	default:
		return ""
	}
}

func toString(i interface{}) string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%v", i)
}
