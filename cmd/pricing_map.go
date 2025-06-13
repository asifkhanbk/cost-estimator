package cmd

// PricingInfo holds the Azure service name and the resource field to use as SKU
type PricingInfo struct {
	ServiceName string
	SKUField    string
}

// ResourcePricingMap maps Terraform resource types to Azure service name and SKU field
var ResourcePricingMap = map[string]PricingInfo{
	// Compute
	"azurerm_linux_virtual_machine":        {ServiceName: "Virtual Machines", SKUField: "size"},
	"azurerm_windows_virtual_machine":      {ServiceName: "Virtual Machines", SKUField: "size"},

	// Kubernetes
	"azurerm_kubernetes_cluster":           {ServiceName: "Kubernetes Service", SKUField: "sku_tier"},
	"azurerm_kubernetes_cluster_node_pool": {ServiceName: "Virtual Machines", SKUField: "vm_size"},

	// Container Registry & Databricks
	"azurerm_container_registry":           {ServiceName: "Container Registry", SKUField: "sku"},
	"azurerm_databricks_workspace":         {ServiceName: "Databricks", SKUField: "sku_name"},

	// Storage
	"azurerm_managed_disk":                 {ServiceName: "Storage", SKUField: "sku_name"},
	"azurerm_disk_encryption_set":          {ServiceName: "Storage", SKUField: "sku_name"},
	"azurerm_storage_account":              {ServiceName: "Storage", SKUField: "account_tier"},
	"azurerm_storage_container":            {ServiceName: "Storage", SKUField: ""},
	"azurerm_storage_blob":                 {ServiceName: "Storage", SKUField: ""},
	"azurerm_blob_data":                    {ServiceName: "Storage", SKUField: ""},
	"azurerm_key_vault":                    {ServiceName: "Key Vault", SKUField: "sku_name"},

	// Backup
	"azurerm_recovery_services_vault":      {ServiceName: "Backup", SKUField: "sku_name"},
	"azurerm_backup_policy_vm":             {ServiceName: "Backup", SKUField: "policy_type"},

	// Networking & CDN
	"azurerm_public_ip":                    {ServiceName: "IP Addresses", SKUField: "sku"},
	"azurerm_virtual_network":              {ServiceName: "Virtual Network", SKUField: ""},
	"azurerm_subnet":                       {ServiceName: "Virtual Network", SKUField: ""},
	"azurerm_network_interface":            {ServiceName: "Network Interface", SKUField: ""},
	"azurerm_network_security_group":       {ServiceName: "Network Security Groups", SKUField: ""},
	"azurerm_nat_gateway":                  {ServiceName: "Virtual Network", SKUField: "sku_name"},
	"azurerm_lb":                           {ServiceName: "Load Balancer", SKUField: "sku"},
	"azurerm_application_gateway":          {ServiceName: "Application Gateway", SKUField: "sku_name"},
	"azurerm_application_gateway_waf_policy":{ServiceName: "Application Gateway", SKUField: "sku_name"},
	"azurerm_firewall":                     {ServiceName: "Azure Firewall", SKUField: "sku_name"},
	"azurerm_cdn_profile":                  {ServiceName: "CDN", SKUField: "sku"},
	"azurerm_data_transfer":                {ServiceName: "Bandwidth", SKUField: ""},

	// DNS
	"azurerm_dns_zone":                     {ServiceName: "DNS", SKUField: ""},
	"azurerm_private_dns_zone":             {ServiceName: "DNS", SKUField: ""},
	"azurerm_private_dns_zone_virtual_network_link": {ServiceName: "DNS", SKUField: ""},
	"azurerm_virtual_network_peering":      {ServiceName: "Virtual Network", SKUField: ""},

	// Identity & Access
	"azurerm_user_assigned_identity":       {ServiceName: "Managed Identities", SKUField: ""},
	"azurerm_role_assignment":              {ServiceName: "Role Based Access Control", SKUField: ""},

	// App Services
	"azurerm_app_service_plan":             {ServiceName: "App Service", SKUField: "sku_name"},
	"azurerm_app_service":                  {ServiceName: "App Service", SKUField: "sku_name"},

	// Databases
	"azurerm_sql_server":                   {ServiceName: "SQL Database", SKUField: "sku_name"},
	"azurerm_sql_database":                 {ServiceName: "SQL Database", SKUField: "sku_name"},
	"azurerm_postgresql_server":            {ServiceName: "Azure Database for PostgreSQL", SKUField: "sku_name"},
	"azurerm_postgresql_flexible_server":   {ServiceName: "Azure Database for PostgreSQL", SKUField: "sku_name"},
	"azurerm_mysql_server":                 {ServiceName: "Azure Database for MySQL", SKUField: "sku_name"},
	"azurerm_mysql_flexible_server":        {ServiceName: "Azure Database for MySQL", SKUField: "sku_name"},
	"azurerm_cosmosdb_account":             {ServiceName: "Azure Cosmos DB", SKUField: "offer_type"},

	// Caching & Messaging
	"azurerm_cache_redis":                  {ServiceName: "Azure Cache for Redis", SKUField: "sku_name"},
	"azurerm_servicebus_namespace":         {ServiceName: "Service Bus", SKUField: "sku"},
	"azurerm_eventhub_namespace":           {ServiceName: "Event Hubs", SKUField: "sku"},
	"azurerm_signalr_service":              {ServiceName: "SignalR", SKUField: "sku"},

	// Integration & API Management
	"azurerm_api_management":               {ServiceName: "API Management", SKUField: "sku_name"},

	// Monitoring & Analytics
	"azurerm_log_analytics_workspace":      {ServiceName: "Log Analytics", SKUField: "sku"},
	"azurerm_application_insights":         {ServiceName: "Application Insights", SKUField: "pricingTier"},
	"azurerm_monitor_diagnostic_setting":   {ServiceName: "Monitoring", SKUField: ""},

	// Automation & DevOps
	"azurerm_automation_account":           {ServiceName: "Automation", SKUField: "sku_name"},
	"azurerm_lab":                          {ServiceName: "Lab Services", SKUField: "sku"},

	// IoT
	"azurerm_iothub":                       {ServiceName: "IoT Hub", SKUField: "sku_name"},

	// Data & Analytics
	"azurerm_data_factory":                 {ServiceName: "Data Factory", SKUField: ""},
	"azurerm_synapse_workspace":            {ServiceName: "Synapse", SKUField: ""},

	// Desktop
	"azurerm_virtual_desktop_host_pool":    {ServiceName: "Virtual Desktop", SKUField: "host_pool_type"},
}
