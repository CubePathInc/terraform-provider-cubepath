package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &objectStorageEventDestinationResource{}
	_ resource.ResourceWithConfigure      = &objectStorageEventDestinationResource{}
	_ resource.ResourceWithImportState    = &objectStorageEventDestinationResource{}
	_ resource.ResourceWithValidateConfig = &objectStorageEventDestinationResource{}
)

func NewObjectStorageEventDestinationResource() resource.Resource {
	return &objectStorageEventDestinationResource{}
}

type objectStorageEventDestinationResource struct {
	client *client.Client
}

type objectStorageEventDestinationModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Type           types.String `tfsdk:"type"`
	URL            types.String `tfsdk:"url"`
	NotificatorID  types.String `tfsdk:"notificator_id"`
	PayloadFormat  types.String `tfsdk:"payload_format"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	SigningSecret  types.String `tfsdk:"signing_secret"`
	URLMasked      types.String `tfsdk:"url_masked"`
	Status         types.String `tfsdk:"status"`
	DisabledReason types.String `tfsdk:"disabled_reason"`
	RulesCount     types.Int64  `tfsdk:"rules_count"`
}

func (r *objectStorageEventDestinationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_storage_event_destination"
}

func (r *objectStorageEventDestinationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a destination for Object Storage event notifications: a signed webhook or a " +
			"Slack or Discord channel of Cloud Alerts. Bucket events reach it through " +
			"cubepath_object_storage_event_rule resources. The signing secret of a webhook is only returned " +
			"when the destination is created and is kept in the Terraform state (marked sensitive): protect " +
			"your state. Import with the destination UUID " +
			"(terraform import cubepath_object_storage_event_destination.example <uuid>); an imported " +
			"destination has no url nor signing_secret in the state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The UUID of the destination.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Description: "Destination name (1 to 64 characters).",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description:   "webhook or notificator (a Cloud Alerts channel). Changing it forces a new destination.",
				Required:      true,
				Validators:    []validator.String{StringOneOf("webhook", "notificator")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"url": schema.StringAttribute{
				Description: "Webhook URL (type webhook): https on port 443 or 8443, resolving to a public address. " +
					"Sensitive: the API only returns it masked (url_masked), so a change made outside Terraform is not detected.",
				Optional:  true,
				Sensitive: true,
			},
			"notificator_id": schema.StringAttribute{
				Description:   "ID of a Slack or Discord channel of Cloud Alerts (type notificator). Changing it forces a new destination.",
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"payload_format": schema.StringAttribute{
				Description: "cubepath (default) or s3 ({\"Records\":[...]} in the AWS shape).",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("cubepath"),
				Validators:  []validator.String{StringOneOf("cubepath", "s3")},
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether events are delivered. Defaults to true. A destination disabled by CubePath " +
					"after repeated failures shows false and is enabled again on the next apply.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"signing_secret": schema.StringAttribute{
				Description: "Secret (whsec_...) that signs every webhook delivery: CubePath-Signature carries " +
					"v1=<hex HMAC-SHA256 of CubePath-Timestamp + \".\" + raw body>. Only known for destinations " +
					"created by Terraform; null for channels and after an import.",
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"url_masked": schema.StringAttribute{
				Description: "The webhook URL as the API shows it, masked.",
				Computed:    true,
			},
			"status": schema.StringAttribute{
				Description: "active, disabled or auto_disabled.",
				Computed:    true,
			},
			"disabled_reason": schema.StringAttribute{
				Description: "Why the destination is disabled: user, failing, admin or abuse.",
				Computed:    true,
			},
			"rules_count": schema.Int64Attribute{
				Description: "Number of rules using the destination.",
				Computed:    true,
			},
		},
	}
}

func (r *objectStorageEventDestinationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	r.client = c
}

func (r *objectStorageEventDestinationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg objectStorageEventDestinationModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || cfg.Type.IsUnknown() || cfg.URL.IsUnknown() || cfg.NotificatorID.IsUnknown() {
		return
	}
	switch cfg.Type.ValueString() {
	case "webhook":
		if cfg.URL.IsNull() || !cfg.NotificatorID.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("url"), "Invalid destination",
				"A webhook destination needs url and no notificator_id.")
		}
	case "notificator":
		if cfg.NotificatorID.IsNull() || !cfg.URL.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("notificator_id"), "Invalid destination",
				"A notificator destination needs notificator_id and no url.")
		}
	}
}

func (r *objectStorageEventDestinationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan objectStorageEventDestinationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.ObjectStorage.CreateEventDestination(ctx, &client.CreateObjectStorageEventDestinationRequest{
		Name:          plan.Name.ValueString(),
		Type:          plan.Type.ValueString(),
		URL:           knownString(plan.URL),
		NotificatorID: knownString(plan.NotificatorID),
		PayloadFormat: plan.PayloadFormat.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Object Storage event destination", err.Error())
		return
	}

	// The secret is never returned again: store it before anything else can fail.
	plan.ID = types.StringValue(created.Destination.UUID)
	plan.SigningSecret = stringOrNull(created.SigningSecret)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("signing_secret"), plan.SigningSecret)...)

	dest := &created.Destination
	if !plan.Enabled.ValueBool() {
		off := false
		dest, err = r.client.ObjectStorage.UpdateEventDestination(ctx, created.Destination.UUID,
			&client.UpdateObjectStorageEventDestinationRequest{Enabled: &off})
		if err != nil {
			resp.Diagnostics.AddError("Error disabling Object Storage event destination", err.Error())
			return
		}
	}
	mapEventDestination(&plan, dest)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *objectStorageEventDestinationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state objectStorageEventDestinationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	dest, err := r.client.ObjectStorage.GetEventDestination(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Object Storage event destination", err.Error())
		return
	}
	if dest.Status == "deleted" {
		resp.State.RemoveResource(ctx)
		return
	}
	mapEventDestination(&state, dest)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *objectStorageEventDestinationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state objectStorageEventDestinationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	upd := eventDestinationUpdate(plan, state)
	dest, err := r.client.ObjectStorage.GetEventDestination(ctx, state.ID.ValueString())
	if err == nil && (upd.Name != nil || upd.URL != nil || upd.PayloadFormat != nil || upd.Enabled != nil) {
		dest, err = r.client.ObjectStorage.UpdateEventDestination(ctx, state.ID.ValueString(), upd)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error updating Object Storage event destination", err.Error())
		return
	}
	plan.ID = state.ID
	plan.SigningSecret = state.SigningSecret
	mapEventDestination(&plan, dest)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// eventDestinationUpdate sends only what changed between the state and the plan.
func eventDestinationUpdate(plan, state objectStorageEventDestinationModel) *client.UpdateObjectStorageEventDestinationRequest {
	upd := &client.UpdateObjectStorageEventDestinationRequest{}
	if !plan.Name.Equal(state.Name) {
		upd.Name = knownString(plan.Name)
	}
	if !plan.URL.Equal(state.URL) && !plan.URL.IsNull() {
		upd.URL = knownString(plan.URL)
	}
	if !plan.PayloadFormat.Equal(state.PayloadFormat) {
		upd.PayloadFormat = knownString(plan.PayloadFormat)
	}
	if !plan.Enabled.Equal(state.Enabled) {
		upd.Enabled = knownBool(plan.Enabled)
	}
	return upd
}

func (r *objectStorageEventDestinationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state objectStorageEventDestinationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Rules are deleted asynchronously: a destination whose last rule is still going away answers
	// 400, so retry for a while.
	err := retryWhile(ctx, accessKeyTimeout, bucketPollInterval, destinationHasRules, func() error {
		return r.client.ObjectStorage.DeleteEventDestination(ctx, state.ID.ValueString())
	})
	if err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Object Storage event destination", err.Error())
	}
}

// destinationHasRules matches the 400 "This destination is used by N event rules" answer.
func destinationHasRules(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && apiErr.IsBadRequest() && strings.Contains(apiErr.Detail, "event rules")
}

func (r *objectStorageEventDestinationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	dest, err := r.client.ObjectStorage.GetEventDestination(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing Object Storage event destination", err.Error())
		return
	}
	state := objectStorageEventDestinationModel{
		Type:          types.StringValue(dest.Type),
		URL:           types.StringNull(),
		NotificatorID: types.StringNull(),
		SigningSecret: types.StringNull(),
	}
	if dest.Notificator != nil {
		state.NotificatorID = types.StringValue(dest.Notificator.ID)
	}
	mapEventDestination(&state, dest)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// mapEventDestination copies the API fields. url keeps the configured value (the API only
// returns it masked) and the secret is never returned after the create.
func mapEventDestination(m *objectStorageEventDestinationModel, d *client.ObjectStorageEventDestination) {
	m.ID = types.StringValue(d.UUID)
	m.Name = types.StringValue(d.Name)
	m.Type = types.StringValue(d.Type)
	m.PayloadFormat = types.StringValue(d.PayloadFormat)
	m.Enabled = types.BoolValue(d.Status == "active")
	m.URLMasked = stringOrNull(d.URLMasked)
	m.Status = types.StringValue(d.Status)
	m.DisabledReason = stringOrNull(d.DisabledReason)
	m.RulesCount = types.Int64Value(int64(d.RulesCount))
	if m.SigningSecret.IsUnknown() {
		m.SigningSecret = types.StringNull()
	}
}
