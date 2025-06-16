// azure/pricing.go
package azure

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "strings"
    "time"

    "github.com/asifkhanbk/cost-estimator/pricing"
)

// NewAzurePricing returns an Azure PricingEngine.
func NewAzurePricing() pricing.PricingEngine {
    return &azurePricing{client: &http.Client{Timeout: 10 * time.Second}}
}

type azurePricing struct {
    client *http.Client
}

func (a *azurePricing) FetchPrice(service, region, sku string) (float64, string, bool) {
    base := "https://prices.azure.com/api/retail/prices"
    var filters []string
    if region != "" && sku != "" {
        filters = append(filters,
            fmt.Sprintf("serviceName eq '%s' and armRegionName eq '%s' and (skuName eq '%s' or armSkuName eq '%s')",
                service, region, sku, sku),
        )
    }
    if region != "" {
        filters = append(filters,
            fmt.Sprintf("serviceName eq '%s' and armRegionName eq '%s'", service, region),
        )
    }
    filters = append(filters, fmt.Sprintf("serviceName eq '%s'", service))

    for _, filter := range filters {
        urlStr := base + "?$filter=" + url.QueryEscape(filter)
        for urlStr != "" {
            resp, err := a.client.Get(urlStr)
            if err != nil {
                return 0, "", false
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
                return 0, "", false
            }

            // Private Endpoint special case
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
                    if strings.Contains(strings.ToLower(item.UnitOfMeasure), "operation") ||
                        strings.Contains(strings.ToLower(item.MeterName), "operation") {
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
