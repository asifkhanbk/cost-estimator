package pricing

// PricingEngine defines a cloud-agnostic price lookup interface.
type PricingEngine interface {
    // FetchPrice returns (unitCost, unitOfMeasure, found).
    FetchPrice(service, region, sku string) (float64, string, bool)
}
