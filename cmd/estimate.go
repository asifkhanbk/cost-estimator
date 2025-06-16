package cmd

import (
    "encoding/json"
    "fmt"
    "os"
    "strings"

    "github.com/olekukonko/tablewriter"
    "github.com/spf13/cobra"

    "github.com/asifkhanbk/cost-estimator/azure"
    "github.com/asifkhanbk/cost-estimator/pricing"
)

var estimateCmd = &cobra.Command{
    Use:   "estimate",
    Short: "Estimate costs from a Terraform JSON plan",
    Run:   runEstimate,
}

func init() {
    rootCmd.AddCommand(estimateCmd)
}

func runEstimate(cmd *cobra.Command, args []string) {
    data, err := os.ReadFile(planFile)
    if err != nil {
        fmt.Printf("❌ Failed to read plan: %v
", err)
        os.Exit(1)
    }

    var plan map[string]interface{}
    if err := json.Unmarshal(data, &plan); err != nil {
        fmt.Printf("❌ Failed to parse JSON: %v
", err)
        os.Exit(1)
    }

    // Extract variables and resources
    vars := extractVariables(plan)
    addrMap := map[string]*Resource{}
    rootMod, ok := plan["planned_values"].(map[string]interface{})["root_module"].(map[string]interface{})
    if !ok {
        fmt.Println("⚠️ No root_module in plan")
        os.Exit(1)
    }
    resources := extractResources(rootMod, addrMap)

    // Build AKS cluster-region map
    clusterRegions := buildClusterRegionMap(resources)

    // Wire in Azure engine
    var engine pricing.PricingEngine = azure.NewAzurePricing()

    table := tablewriter.NewWriter(os.Stdout)
    table.SetHeader([]string{"Type", "Name", "Region", "SKU / Detail", "Unit", "Usage", "Unit Cost", "Monthly Cost"})

    var total float64
    for _, r := range resources {
        def, found := ResourceTypePricingMap[r.Type]
        if !found {
            def = fallbackPricingDefinition(r.Type)
        }

        // Resolve region & SKU
        region := resolveRegion(r, def, vars, addrMap, clusterRegions)
        sku := resolveSKU(r, def, vars, addrMap)
        quantity, quantityDesc := extractUsage(r, def)

        // Fetch pricing
        unitCost, unit, foundPrice := engine.FetchPrice(def.ServiceName, region, sku)
        if r.Type == "azurerm_private_endpoint" && !foundPrice {
            // Fallback
            unitCost = 0.01
            unit = "1 Hour"
            foundPrice = true
        }

        // Compute monthly cost according to original logic
        var monthlyCost float64
        usageText := quantityDesc
        lowerUnit := strings.ToLower(unit)
        if foundPrice {
            if strings.Contains(lowerUnit, "hour") {
                monthlyCost = unitCost * 730 * quantity
                if usageText == "-" {
                    usageText = fmt.Sprintf("%.0f x 730 hours", quantity)
                }
            } else if strings.Contains(lowerUnit, "gb") && quantity > 0 {
                monthlyCost = unitCost * quantity
            } else if strings.Contains(lowerUnit, "operation") && quantity > 0 {
                monthlyCost = unitCost * quantity
            } else {
                monthlyCost = unitCost * quantity
            }
        }

        total += monthlyCost
        table.Append([]string{
            r.Type,
            r.Name,
            region,
            sku,
            unit,
            usageText,
            fmt.Sprintf("%.6f", unitCost),
            fmt.Sprintf("%.2f", monthlyCost),
        })
    }

    table.Render()
    fmt.Printf("
💰 Total Estimated Monthly Cost: $%.2f
", total)
}