package provider

import (
	"context"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &objectStorageReplicationsDataSource{}
	_ datasource.DataSourceWithConfigure = &objectStorageReplicationsDataSource{}
)

func NewObjectStorageReplicationsDataSource() datasource.DataSource {
	return &objectStorageReplicationsDataSource{}
}

type objectStorageReplicationsDataSource struct {
	client *client.Client
}

type objectStorageReplicationsDataSourceModel struct {
	ID           types.String                  `tfsdk:"id"`
	Direction    types.String                  `tfsdk:"direction"`
	BucketUUID   types.String                  `tfsdk:"bucket_uuid"`
	Replications []objectStorageReplicationRow `tfsdk:"replications"`
}

type objectStorageReplicationRow struct {
	UUID                        types.String `tfsdk:"uuid"`
	Status                      types.String `tfsdk:"status"`
	PauseReason                 types.String `tfsdk:"pause_reason"`
	Direction                   types.String `tfsdk:"direction"`
	Enabled                     types.Bool   `tfsdk:"enabled"`
	SourceBucketUUID            types.String `tfsdk:"source_bucket_uuid"`
	SourceBucketName            types.String `tfsdk:"source_bucket_name"`
	SourceOrganizationName      types.String `tfsdk:"source_organization_name"`
	DestinationType             types.String `tfsdk:"destination_type"`
	DestinationBucketUUID       types.String `tfsdk:"destination_bucket_uuid"`
	DestinationBucketName       types.String `tfsdk:"destination_bucket_name"`
	DestinationOrganizationName types.String `tfsdk:"destination_organization_name"`
	DestinationProvider         types.String `tfsdk:"destination_provider"`
	DestinationEndpoint         types.String `tfsdk:"destination_endpoint"`
	DestinationRegion           types.String `tfsdk:"destination_region"`
	DestinationBucket           types.String `tfsdk:"destination_bucket"`
	Prefix                      types.String `tfsdk:"prefix"`
	DeleteMarkerReplication     types.Bool   `tfsdk:"delete_marker_replication"`
	DeleteReplication           types.Bool   `tfsdk:"delete_replication"`
	ExistingObjects             types.Bool   `tfsdk:"existing_objects"`
	Health                      types.String `tfsdk:"health"`
	HealthReason                types.String `tfsdk:"health_reason"`
	BackfillStatus              types.String `tfsdk:"backfill_status"`
	BackfillObjects             types.Int64  `tfsdk:"backfill_objects"`
	BackfillBytes               types.Int64  `tfsdk:"backfill_bytes"`
	CreatedAt                   types.String `tfsdk:"created_at"`
}

func (d *objectStorageReplicationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_storage_replications"
}

func (d *objectStorageReplicationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	str := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Description: desc, Computed: true}
	}
	boolean := func(desc string) schema.BoolAttribute {
		return schema.BoolAttribute{Description: desc, Computed: true}
	}
	num := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{Description: desc, Computed: true}
	}
	resp.Schema = schema.Schema{
		Description: "Lists the Object Storage replications of the organization: outgoing (from your buckets) and " +
			"incoming (into your buckets from another organization), newest first.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"direction": schema.StringAttribute{
				Description: "outgoing, incoming or all (default). With all, a replication between two buckets of your " +
					"organization is listed once, as outgoing.",
				Optional:   true,
				Validators: []validator.String{StringOneOf("outgoing", "incoming", "all")},
			},
			"bucket_uuid": schema.StringAttribute{
				Description: "Only the replications whose source (outgoing) or destination (incoming) is this bucket.",
				Optional:    true,
			},
			"replications": schema.ListNestedAttribute{
				Description: "The replications.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid":                          str("Replication UUID."),
						"status":                        str("pending, active, paused, suspended, error or deleting."),
						"pause_reason":                  str("Why it is paused, if it is."),
						"direction":                     str("outgoing or incoming."),
						"enabled":                       boolean("Whether the replication rule is enabled."),
						"source_bucket_uuid":            str("Source bucket UUID (null for an incoming replication of another organization)."),
						"source_bucket_name":            str("Source bucket name."),
						"source_organization_name":      str("Organization of the source bucket."),
						"destination_type":              str("cubepath or external."),
						"destination_bucket_uuid":       str("cubepath: destination bucket UUID."),
						"destination_bucket_name":       str("cubepath: destination bucket name."),
						"destination_organization_name": str("cubepath: organization of the destination bucket."),
						"destination_provider":          str("external: aws, wasabi or other."),
						"destination_endpoint":          str("external: endpoint host."),
						"destination_region":            str("external: region."),
						"destination_bucket":            str("external: bucket name."),
						"prefix":                        str("Prefix filter, if any."),
						"delete_marker_replication":     boolean("Whether delete markers are replicated."),
						"delete_replication":            boolean("Whether deletes of a specific version are replicated."),
						"existing_objects":              boolean("Whether the objects the bucket already held are copied."),
						"health":                        str("unknown, ok, lagging or failing."),
						"health_reason":                 str("Reason of a failing health."),
						"backfill_status":               str("none, queued, running, completed or failed."),
						"backfill_objects":              num("Objects copied by the backfills."),
						"backfill_bytes":                num("Bytes copied by the backfills."),
						"created_at":                    str("Creation time."),
					},
				},
			},
		},
	}
}

func (d *objectStorageReplicationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureObjectStorageDataSource(req, resp); c != nil {
		d.client = c
	}
}

func (d *objectStorageReplicationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state objectStorageReplicationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	repls, err := d.client.ObjectStorage.ListReplications(ctx, state.Direction.ValueString(), state.BucketUUID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading Object Storage replications", err.Error())
		return
	}
	state.ID = types.StringValue("object_storage_replications")
	state.Replications = replicationRows(repls)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func replicationRows(repls []client.ObjectStorageReplication) []objectStorageReplicationRow {
	rows := make([]objectStorageReplicationRow, 0, len(repls))
	for _, r := range repls {
		dst := r.Destination
		rows = append(rows, objectStorageReplicationRow{
			UUID:                        types.StringValue(r.UUID),
			Status:                      types.StringValue(r.Status),
			PauseReason:                 stringOrNull(r.PauseReason),
			Direction:                   types.StringValue(r.Direction),
			Enabled:                     types.BoolValue(r.Rules.Enabled),
			SourceBucketUUID:            stringOrNull(r.Source.BucketUUID),
			SourceBucketName:            stringOrNull(r.Source.BucketName),
			SourceOrganizationName:      stringOrNull(r.Source.OrganizationName),
			DestinationType:             types.StringValue(dst.Type),
			DestinationBucketUUID:       stringOrNull(dst.BucketUUID),
			DestinationBucketName:       stringOrNull(dst.BucketName),
			DestinationOrganizationName: stringOrNull(dst.OrganizationName),
			DestinationProvider:         stringOrNull(dst.Provider),
			DestinationEndpoint:         stringOrNull(dst.Endpoint),
			DestinationRegion:           stringOrNull(dst.Region),
			DestinationBucket:           stringOrNull(dst.Bucket),
			Prefix:                      stringOrNull(r.Rules.Prefix),
			DeleteMarkerReplication:     types.BoolValue(r.Rules.DeleteMarkerReplication),
			DeleteReplication:           types.BoolValue(r.Rules.DeleteReplication),
			ExistingObjects:             types.BoolValue(r.Rules.ExistingObjects),
			Health:                      types.StringValue(r.Health),
			HealthReason:                stringOrNull(r.HealthReason),
			BackfillStatus:              types.StringValue(r.Backfill.Status),
			BackfillObjects:             types.Int64Value(r.Backfill.Objects),
			BackfillBytes:               types.Int64Value(r.Backfill.Bytes),
			CreatedAt:                   stringOrNull(r.CreatedAt),
		})
	}
	return rows
}
