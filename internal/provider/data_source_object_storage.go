package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &objectStorageTiersDataSource{}
	_ datasource.DataSourceWithConfigure = &objectStorageTiersDataSource{}
	_ datasource.DataSource              = &objectStorageBucketDataSource{}
	_ datasource.DataSourceWithConfigure = &objectStorageBucketDataSource{}
	_ datasource.DataSource              = &objectStorageUsageDataSource{}
	_ datasource.DataSourceWithConfigure = &objectStorageUsageDataSource{}
)

// configureObjectStorageDataSource extracts the API client for the Object Storage data sources.
func configureObjectStorageDataSource(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return nil
	}
	return c
}

// ---------------------------------------------------------------------------
// cubepath_object_storage_tiers
// ---------------------------------------------------------------------------

func NewObjectStorageTiersDataSource() datasource.DataSource {
	return &objectStorageTiersDataSource{}
}

type objectStorageTiersDataSource struct {
	client *client.Client
}

type objectStorageTiersDataSourceModel struct {
	ID    types.String             `tfsdk:"id"`
	Tiers []objectStorageTierModel `tfsdk:"tiers"`
}

type objectStorageTierModel struct {
	UUID                types.String  `tfsdk:"uuid"`
	Slug                types.String  `tfsdk:"slug"`
	Name                types.String  `tfsdk:"name"`
	Media               types.String  `tfsdk:"media"`
	LocationName        types.String  `tfsdk:"location_name"`
	LocationDescription types.String  `tfsdk:"location_description"`
	Region              types.String  `tfsdk:"region"`
	Endpoint            types.String  `tfsdk:"endpoint"`
	PriceStorageGBMonth types.Float64 `tfsdk:"price_storage_gb_month"`
	PriceEgressGB       types.Float64 `tfsdk:"price_egress_gb"`
	PriceClassAPer1K    types.Float64 `tfsdk:"price_class_a_per_1k"`
	PriceClassBPer1K    types.Float64 `tfsdk:"price_class_b_per_1k"`
	FreeStorageGBMonth  types.Float64 `tfsdk:"free_storage_gb_month"`
	FreeEgressGB        types.Float64 `tfsdk:"free_egress_gb"`
	FreeRequests        types.Int64   `tfsdk:"free_requests"`
	AcceptingNew        types.Bool    `tfsdk:"accepting_new"`
}

func (d *objectStorageTiersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_storage_tiers"
}

func (d *objectStorageTiersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	computed := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Description: desc, Computed: true}
	}
	price := func(desc string) schema.Float64Attribute {
		return schema.Float64Attribute{Description: desc, Computed: true}
	}
	resp.Schema = schema.Schema{
		Description: "Fetches the available Object Storage tiers with their endpoint, prices and free tier.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"tiers": schema.ListNestedAttribute{
				Description: "Available storage tiers.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid":                   computed("Tier UUID."),
						"slug":                   computed("Tier slug, usable as tier in buckets and keys (for example infrequent_access)."),
						"name":                   computed("Tier name."),
						"media":                  computed("Storage media (hdd, ssd)."),
						"location_name":          computed("Location code of the storage cluster."),
						"location_description":   computed("Location description."),
						"region":                 computed("S3 region."),
						"endpoint":               computed("S3 endpoint."),
						"price_storage_gb_month": price("USD per GiB stored per month."),
						"price_egress_gb":        price("USD per GiB of egress."),
						"price_class_a_per_1k":   price("USD per 1,000 class A requests (writes and listings)."),
						"price_class_b_per_1k":   price("USD per 1,000 class B requests (reads)."),
						"free_storage_gb_month":  price("GiB-month included for free every month, per organization."),
						"free_egress_gb":         price("GiB of egress included for free every month, per organization."),
						"free_requests": schema.Int64Attribute{
							Description: "Requests included for free every month, per organization.",
							Computed:    true,
						},
						"accepting_new": schema.BoolAttribute{
							Description: "Whether the tier accepts new buckets.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *objectStorageTiersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureObjectStorageDataSource(req, resp); c != nil {
		d.client = c
	}
}

func (d *objectStorageTiersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tiers, err := d.client.ObjectStorage.ListTiers(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Object Storage tiers", err.Error())
		return
	}

	state := objectStorageTiersDataSourceModel{ID: types.StringValue("object_storage_tiers")}
	state.Tiers = make([]objectStorageTierModel, 0, len(tiers))
	for _, t := range tiers {
		state.Tiers = append(state.Tiers, objectStorageTierModel{
			UUID:                types.StringValue(t.UUID),
			Slug:                types.StringValue(t.Slug),
			Name:                types.StringValue(t.Name),
			Media:               types.StringValue(t.Media),
			LocationName:        types.StringValue(t.LocationName),
			LocationDescription: types.StringValue(t.LocationDescription),
			Region:              types.StringValue(t.Region),
			Endpoint:            types.StringValue(t.Endpoint),
			PriceStorageGBMonth: types.Float64Value(t.Prices.StorageGBMonth),
			PriceEgressGB:       types.Float64Value(t.Prices.EgressGB),
			PriceClassAPer1K:    types.Float64Value(t.Prices.ClassAPer1K),
			PriceClassBPer1K:    types.Float64Value(t.Prices.ClassBPer1K),
			FreeStorageGBMonth:  types.Float64Value(t.FreeTier.StorageGBMonth),
			FreeEgressGB:        types.Float64Value(t.FreeTier.EgressGB),
			FreeRequests:        types.Int64Value(t.FreeTier.Requests),
			AcceptingNew:        types.BoolValue(t.AcceptingNew),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ---------------------------------------------------------------------------
// cubepath_object_storage_bucket
// ---------------------------------------------------------------------------

func NewObjectStorageBucketDataSource() datasource.DataSource {
	return &objectStorageBucketDataSource{}
}

type objectStorageBucketDataSource struct {
	client *client.Client
}

type objectStorageBucketDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Status         types.String `tfsdk:"status"`
	ProjectID      types.Int64  `tfsdk:"project_id"`
	TierSlug       types.String `tfsdk:"tier_slug"`
	TierUUID       types.String `tfsdk:"tier_uuid"`
	LocationName   types.String `tfsdk:"location_name"`
	Region         types.String `tfsdk:"region"`
	Endpoint       types.String `tfsdk:"endpoint"`
	PathStyleURL   types.String `tfsdk:"path_style_url"`
	VirtualHostURL types.String `tfsdk:"virtual_host_url"`
	Versioning     types.String `tfsdk:"versioning"`
	Protected      types.Bool   `tfsdk:"protected"`
	WriteBlocked   types.Bool   `tfsdk:"write_blocked"`
	SizeBytes      types.Int64  `tfsdk:"size_bytes"`
	ObjectsCount   types.Int64  `tfsdk:"objects_count"`
	CDNConnected   types.Bool   `tfsdk:"cdn_connected"`
	CDNZoneUUID    types.String `tfsdk:"cdn_zone_uuid"`
	CDNOriginUUID  types.String `tfsdk:"cdn_origin_uuid"`
	CDNDomain      types.String `tfsdk:"cdn_domain"`
	Tags           types.Map    `tfsdk:"tags"`

	ObjectLockEnabled          types.Bool   `tfsdk:"object_lock_enabled"`
	ObjectLockDefaultRetention types.Object `tfsdk:"object_lock_default_retention"`
	LockedContentKept          types.Bool   `tfsdk:"locked_content_kept"`
	Encryption                 types.Object `tfsdk:"encryption"`
}

func (d *objectStorageBucketDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_storage_bucket"
}

func (d *objectStorageBucketDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	str := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Description: desc, Computed: true}
	}
	resp.Schema = schema.Schema{
		Description: "Fetches an Object Storage bucket by UUID or by name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Bucket UUID. Set this or name.",
				Optional:    true,
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "Bucket name. Set this or id.",
				Optional:    true,
				Computed:    true,
			},
			"status":              str("Bucket status."),
			"project_id":          schema.Int64Attribute{Description: "Project ID.", Computed: true},
			"tier_slug":           str("Tier slug."),
			"tier_uuid":           str("Tier UUID."),
			"location_name":       str("Location of the storage cluster."),
			"region":              str("S3 region."),
			"endpoint":            str("S3 endpoint."),
			"path_style_url":      str("Path-style URL of the bucket (endpoint/bucket)."),
			"virtual_host_url":    str("Virtual-hosted URL of the bucket (bucket.endpoint host)."),
			"versioning":          str("Versioning: off, enabled or suspended."),
			"protected":           schema.BoolAttribute{Description: "Deletion protection.", Computed: true},
			"write_blocked":       schema.BoolAttribute{Description: "True while uploads to the bucket are paused.", Computed: true},
			"size_bytes":          schema.Int64Attribute{Description: "Stored size in bytes (every version), refreshed every 15 minutes.", Computed: true},
			"objects_count":       schema.Int64Attribute{Description: "Number of objects, refreshed every 15 minutes.", Computed: true},
			"cdn_connected":       schema.BoolAttribute{Description: "True while a CDN origin serves the bucket.", Computed: true},
			"cdn_zone_uuid":       str("UUID of the CDN zone serving the bucket, if any."),
			"cdn_origin_uuid":     str("UUID of the CDN origin serving the bucket, if any."),
			"cdn_domain":          str("CDN domain serving the bucket, if any."),
			"tags":                schema.MapAttribute{Description: "Bucket tags as key = value (empty when none).", ElementType: types.StringType, Computed: true},
			"object_lock_enabled": schema.BoolAttribute{Description: "True when the bucket was created with Object Lock.", Computed: true},
			"object_lock_default_retention": schema.SingleNestedAttribute{
				Description: "Default retention of the bucket (null when it has none).",
				Computed:    true,
				Attributes: map[string]schema.Attribute{
					"mode":  str("governance or compliance."),
					"days":  schema.Int64Attribute{Description: "Retention in days (null when set in years).", Computed: true},
					"years": schema.Int64Attribute{Description: "Retention in years (null when set in days).", Computed: true},
				},
			},
			"locked_content_kept": schema.BoolAttribute{
				Description: "True when the last delete left object versions protected by Object Lock in the bucket.",
				Computed:    true,
			},
			"encryption": schema.SingleNestedAttribute{
				Description: encryptionDescription,
				Computed:    true,
				Attributes: map[string]schema.Attribute{
					"algorithm": str("AES256."),
					"scope":     str("all_objects or new_objects."),
				},
			},
		},
	}
}

func (d *objectStorageBucketDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureObjectStorageDataSource(req, resp); c != nil {
		d.client = c
	}
}

func (d *objectStorageBucketDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config objectStorageBucketDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := config.ID.ValueString()
	name := config.Name.ValueString()
	if (uuid == "") == (name == "") {
		resp.Diagnostics.AddError("Invalid bucket lookup", "Set exactly one of id or name.")
		return
	}

	if uuid == "" {
		buckets, err := d.client.ObjectStorage.ListBuckets(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Error listing Object Storage buckets", err.Error())
			return
		}
		for _, b := range buckets {
			if b.Name == name {
				uuid = b.UUID
				break
			}
		}
		if uuid == "" {
			resp.Diagnostics.AddError("Bucket not found", fmt.Sprintf("No bucket named %q in this organization.", name))
			return
		}
	}

	bucket, err := d.client.ObjectStorage.GetBucket(ctx, uuid)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.Diagnostics.AddError("Bucket not found", fmt.Sprintf("No bucket with UUID %q in this organization.", uuid))
			return
		}
		resp.Diagnostics.AddError("Error reading Object Storage bucket", err.Error())
		return
	}

	state := objectStorageBucketDataSourceModel{
		ID:            types.StringValue(bucket.UUID),
		Name:          types.StringValue(bucket.Name),
		Status:        types.StringValue(bucket.Status),
		ProjectID:     types.Int64Null(),
		TierSlug:      types.StringValue(bucket.Tier.Slug),
		TierUUID:      types.StringValue(bucket.Tier.UUID),
		LocationName:  types.StringValue(bucket.LocationName),
		Region:        types.StringValue(bucket.Region),
		Endpoint:      types.StringValue(bucket.Endpoint),
		Versioning:    types.StringValue(bucket.Versioning),
		Protected:     types.BoolValue(bucket.Protected),
		WriteBlocked:  types.BoolValue(bucket.WriteBlocked),
		SizeBytes:     types.Int64Value(bucket.SizeBytes),
		ObjectsCount:  types.Int64Value(bucket.ObjectsCount),
		CDNConnected:  types.BoolValue(bucket.CDNConnected),
		PathStyleURL:  types.StringNull(),
		CDNZoneUUID:   types.StringNull(),
		CDNOriginUUID: types.StringNull(),
		CDNDomain:     types.StringNull(),
		Tags:          tagsToMap(bucket.Tags),

		ObjectLockEnabled:          types.BoolValue(bucket.ObjectLock.Enabled),
		ObjectLockDefaultRetention: retentionToObject(bucket.ObjectLock.DefaultRetention),
		LockedContentKept:          types.BoolValue(bucket.LockedContentKept),
		Encryption:                 encryptionToObject(bucket.Encryption),
	}
	state.VirtualHostURL = types.StringNull()
	if bucket.ProjectID != nil {
		state.ProjectID = types.Int64Value(int64(*bucket.ProjectID))
	}
	if bucket.Connection != nil {
		state.PathStyleURL = types.StringValue(bucket.Connection.PathStyleURL)
		state.VirtualHostURL = types.StringValue(bucket.Connection.VirtualHostURL)
	}
	if bucket.CDN != nil {
		state.CDNZoneUUID = types.StringValue(bucket.CDN.ZoneUUID)
		state.CDNOriginUUID = types.StringValue(bucket.CDN.OriginUUID)
		domain := bucket.CDN.Domain
		if bucket.CDN.CustomDomain != "" {
			domain = bucket.CDN.CustomDomain
		}
		state.CDNDomain = types.StringValue(domain)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ---------------------------------------------------------------------------
// cubepath_object_storage_usage
// ---------------------------------------------------------------------------

func NewObjectStorageUsageDataSource() datasource.DataSource {
	return &objectStorageUsageDataSource{}
}

type objectStorageUsageDataSource struct {
	client *client.Client
}

type objectStorageUsageDataSourceModel struct {
	ID               types.String                    `tfsdk:"id"`
	Period           types.String                    `tfsdk:"period"`
	ProjectID        types.Int64                     `tfsdk:"project_id"`
	MetricsAvailable types.Bool                      `tfsdk:"metrics_available"`
	TotalCost        types.Float64                   `tfsdk:"total_cost"`
	ProjectedCost    types.Float64                   `tfsdk:"projected_cost"`
	Buckets          []objectStorageUsageBucketModel `tfsdk:"buckets"`
}

type objectStorageUsageBucketModel struct {
	UUID            types.String  `tfsdk:"uuid"`
	Name            types.String  `tfsdk:"name"`
	StorageGiBMonth types.Float64 `tfsdk:"storage_gib_month"`
	EgressBytes     types.Int64   `tfsdk:"egress_bytes"`
	ClassARequests  types.Int64   `tfsdk:"class_a_requests"`
	ClassBRequests  types.Int64   `tfsdk:"class_b_requests"`
	Cost            types.Float64 `tfsdk:"cost"`
}

func (d *objectStorageUsageDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_storage_usage"
}

func (d *objectStorageUsageDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the Object Storage usage and cost of a month.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"period": schema.StringAttribute{
				Description: "Month in YYYY-MM format, at most 12 months back. Defaults to the current month.",
				Optional:    true,
				Computed:    true,
			},
			"project_id": schema.Int64Attribute{
				Description: "Only include this project's buckets.",
				Optional:    true,
			},
			"metrics_available": schema.BoolAttribute{
				Description: "False when usage metrics are temporarily unavailable (costs are still returned).",
				Computed:    true,
			},
			"total_cost": schema.Float64Attribute{
				Description: "USD charged so far in the month.",
				Computed:    true,
			},
			"projected_cost": schema.Float64Attribute{
				Description: "USD projected for the whole month.",
				Computed:    true,
			},
			"buckets": schema.ListNestedAttribute{
				Description: "Usage per bucket. Quantities are null when metrics are unavailable.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid":              schema.StringAttribute{Description: "Bucket UUID.", Computed: true},
						"name":              schema.StringAttribute{Description: "Bucket name.", Computed: true},
						"storage_gib_month": schema.Float64Attribute{Description: "GiB-month stored.", Computed: true},
						"egress_bytes":      schema.Int64Attribute{Description: "Billable egress in bytes.", Computed: true},
						"class_a_requests":  schema.Int64Attribute{Description: "Class A requests.", Computed: true},
						"class_b_requests":  schema.Int64Attribute{Description: "Class B requests.", Computed: true},
						"cost":              schema.Float64Attribute{Description: "USD charged for the bucket in the month.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *objectStorageUsageDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureObjectStorageDataSource(req, resp); c != nil {
		d.client = c
	}
}

func (d *objectStorageUsageDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config objectStorageUsageDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var projectID *int
	if !config.ProjectID.IsNull() {
		pid := int(config.ProjectID.ValueInt64())
		projectID = &pid
	}
	usage, err := d.client.ObjectStorage.GetUsage(ctx, config.Period.ValueString(), projectID, "")
	if err != nil {
		resp.Diagnostics.AddError("Error reading Object Storage usage", err.Error())
		return
	}

	state := objectStorageUsageDataSourceModel{
		ID:               types.StringValue("object_storage_usage_" + usage.Period),
		Period:           types.StringValue(usage.Period),
		ProjectID:        config.ProjectID,
		MetricsAvailable: types.BoolValue(usage.MetricsAvailable),
		TotalCost:        types.Float64Value(usage.TotalCost),
		ProjectedCost:    types.Float64Value(usage.ProjectedCost),
		Buckets:          make([]objectStorageUsageBucketModel, 0, len(usage.Buckets)),
	}
	for _, b := range usage.Buckets {
		state.Buckets = append(state.Buckets, objectStorageUsageBucketModel{
			UUID:            types.StringValue(b.UUID),
			Name:            types.StringValue(b.Name),
			StorageGiBMonth: types.Float64PointerValue(b.StorageGiBMonth),
			EgressBytes:     types.Int64PointerValue(b.EgressBytes),
			ClassARequests:  types.Int64PointerValue(b.ClassARequests),
			ClassBRequests:  types.Int64PointerValue(b.ClassBRequests),
			Cost:            types.Float64Value(b.Cost),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
