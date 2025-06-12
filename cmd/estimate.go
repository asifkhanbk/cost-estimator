package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	// "strconv"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var planFile string
var providerFilter string

var resourcePricingMap = map[string]struct {
	ServiceName string
	SKUField    string
}{
	"azurerm_linux_virtual_machine":     {"Virtual Machines", "size"},
	"azurerm_windows_virtual_machine":   {"Virtual Machines", "size"},
	"azurerm_key_vault":                 {"Key Vault", "sku_name"},
	"azurerm_private_endpoint":          {"Private Link", ""},
	"azurerm_storage_account":           {"Storage", "account_tier"},
	"azurerm_app_service_plan":          {"App Service", "sku_name"},
	"azurerm_sql_database":              {"SQL Database", "sku_name"},
	"azurerm_redis_cache":               {"Azure Cache for Redis", "sku_name"},
	"azurerm_postgresql_flexible_server": {"Azure Database for PostgreSQL", "sku_name"},
	"azurerm_mysql_flexible_server":     {"Azure Database for MySQL", "sku_name"},
	"azurerm_kubernetes_cluster":        {"Kubernetes Service", "sku_tier"},
	"azurerm_network_interface":         {"Network Interface", ""},
	"azurerm_network_security_group":    {"Network Security Groups", ""},
	"azurerm_public_ip":                 {"IP Addresses", "sku"},
	"azurerm_virtual_network":           {"Virtual Network", ""},
	"azurerm_subnet":                    {"Virtual Network", ""},
	"azurerm_managed_disk":              {"Storage", "sku_name"},
	"azurerm_nat_gateway":               {"Virtual Network", "sku_name"},
	"azurerm_firewall":                  {"Azure Firewall", "sku_name"},
	"azurerm_application_gateway":       {"Application Gateway", "sku_name"},
	"azurerm_lb":                        {"Load Balancer", "sku"},
	"azurerm_data_transfer":             {"Bandwidth", ""},
	"azurerm_application_insights":      {"Application Insights", "pricingTier"},
	"azurerm_cosmosdb_account":          {"Azure Cosmos DB", "offer_type"},
}

var estimateCmd = &cobra.Command{
	Use:   "estimate",
	Short: "Estimate infra resources from a Terraform plan",
	Long:  `Parse a Terraform JSON plan and estimate monthly cost using Azure Retail Prices API.`,
	Run: func(cmd *cobra.Command, args []string) {
		if planFile == "" {
			fmt.Println("❌ Please provide a plan file using --plan or -p")
			return
		}

		data, err := os.ReadFile(planFile)
		if err != nil {
			fmt.Printf("❌ Failed to read file: %v\n", err)
			return
		}

		var plan map[string]interface{}
		if err := json.Unmarshal(data, &plan); err != nil {
			fmt.Printf("❌ Failed to parse JSON: %v\n", err)
			return
		}

		if planned, ok := plan["planned_values"].(map[string]interface{}); ok {
			if rootModule, ok := planned["root_module"].(map[string]interface{}); ok {
				resources := extractResources(rootModule)
				if len(resources) == 0 {
					fmt.Println("⚠️ No resources found in the plan.")
					return
				}

				table := tablewriter.NewWriter(os.Stdout)
				table.Header([]string{"Type", "Name", "Location", "Raw SKU", "Normalized SKU", "Monthly Cost ($)"})

				for _, res := range resources {
					typeVal := res["type"].(string)
					name := res["name"].(string)

					if providerFilter != "" && !strings.HasPrefix(typeVal, providerFilter+"_") {
						continue
					}

					val := res["values"].(map[string]interface{})
					location := fmt.Sprintf("%v", val["location"])

					pricingInfo, ok := resourcePricingMap[typeVal]
					if !ok {
						table.Append([]string{typeVal, name, location, "-", "-", "N/A (no pricing map)"})
						continue
					}

					rawSKU := ""
					if pricingInfo.SKUField != "" {
						rawSKU = fmt.Sprintf("%v", val[pricingInfo.SKUField])
					}
					sku := normalizeSku(typeVal, rawSKU)

					price, err := queryAzurePrice(location, sku, pricingInfo.ServiceName, typeVal, val)
					monthlyCost := "N/A"
					if err == nil {
						monthlyCost = fmt.Sprintf("%.2f", price*730)
					} else if typeVal == "azurerm_virtual_network" || typeVal == "azurerm_subnet" || typeVal == "azurerm_network_security_group" {
						monthlyCost = "Free"
					}

					table.Append([]string{typeVal, name, location, rawSKU, sku, monthlyCost})
				}

				table.Render()
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(estimateCmd)
	estimateCmd.Flags().StringVarP(&planFile, "plan", "p", "", "Path to Terraform plan JSON")
	estimateCmd.Flags().StringVar(&providerFilter, "provider", "", "Filter by provider prefix (e.g., azurerm)")
}

func extractResources(module map[string]interface{}) []map[string]interface{} {
	var result []map[string]interface{}

	if resources, ok := module["resources"].([]interface{}); ok {
		for _, r := range resources {
			if resMap, ok := r.(map[string]interface{}); ok {
				typeVal := fmt.Sprintf("%v", resMap["type"])
				name := fmt.Sprintf("%v", resMap["name"])
				values, _ := resMap["values"].(map[string]interface{})

				result = append(result, map[string]interface{}{
					"type":   typeVal,
					"name":   name,
					"values": values,
				})
			}
		}
	}

	if children, ok := module["child_modules"].([]interface{}); ok {
		for _, child := range children {
			if childMap, ok := child.(map[string]interface{}); ok {
				childResources := extractResources(childMap)
				result = append(result, childResources...)
			}
		}
	}

	return result
}

func normalizeSku(resourceType, raw string) string {
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(resourceType, "azurerm_linux_virtual_machine") || strings.HasPrefix(resourceType, "azurerm_windows_virtual_machine") {
		raw = strings.TrimPrefix(raw, "Standard_")
		return strings.ReplaceAll(raw, "_", " ")
	}
	return strings.Title(strings.ReplaceAll(raw, "_", " "))
}

func queryAzurePrice(region, sku, service, resourceType string, values map[string]interface{}) (float64, error) {
	filter := fmt.Sprintf("$filter=armRegionName eq '%s' and serviceName eq '%s'", region, service)
	if sku != "" {
		filter += fmt.Sprintf(" and skuName eq '%s'", sku)
	}

	url := fmt.Sprintf("https://prices.azure.com/api/retail/prices?%s", filter)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}

	if items, ok := data["Items"].([]interface{}); ok {
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				meterName := fmt.Sprintf("%v", m["meterName"])
				unit := fmt.Sprintf("%v", m["unitOfMeasure"])
				price, _ := m["retailPrice"].(float64)

				switch resourceType {
				case "azurerm_public_ip":
					if (strings.Contains(meterName, "Public IP") || strings.Contains(meterName, "IPv4") || strings.Contains(meterName, "IP Address")) && unit == "1 Hour" {
						return price, nil
					}
				case "azurerm_managed_disk":
					if unit == "1 GB/Month" {
						size := 128
						if sz, ok := values["disk_size_gb"].(float64); ok {
							size = int(sz)
						}
						return price * float64(size), nil
					}
				case "azurerm_data_transfer":
					if strings.Contains(meterName, "Data Transfer Out") && unit == "1 GB" {
						return price * 100, nil
					}
				case "azurerm_application_insights":
					if strings.Contains(meterName, "Data Point") && unit == "1 Million" {
						return price * 1, nil
					}
				case "azurerm_cosmosdb_account":
					if strings.Contains(meterName, "100 RU/s") && unit == "1 Unit" {
						return price * 4, nil
					}
				default:
					if price > 0 {
						return price, nil
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("no price found for region=%s sku=%s", region, sku)
}
