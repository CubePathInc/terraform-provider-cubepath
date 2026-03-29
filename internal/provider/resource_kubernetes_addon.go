package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &kubernetesAddonResource{}
	_ resource.ResourceWithConfigure   = &kubernetesAddonResource{}
	_ resource.ResourceWithImportState = &kubernetesAddonResource{}
)

func NewKubernetesAddonResource() resource.Resource {
	return &kubernetesAddonResource{}
}

type kubernetesAddonResource struct {
	client *client.Client
}

type kubernetesAddonResourceModel struct {
	ID          types.String `tfsdk:"id"`
	ClusterID   types.String `tfsdk:"cluster_id"`
	Slug        types.String `tfsdk:"slug"`
	Values      types.String `tfsdk:"values"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Category    types.String `tfsdk:"category"`
	Namespace   types.String `tfsdk:"namespace"`
	HelmChart   types.String `tfsdk:"helm_chart"`
	Status      types.String `tfsdk:"status"`
	Version     types.String `tfsdk:"version"`
	InstalledAt types.String `tfsdk:"installed_at"`
}

func (r *kubernetesAddonResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_addon"
}

func (r *kubernetesAddonResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Kubernetes addon installation on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The UUID of the installed addon.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cluster_id": schema.StringAttribute{
				Description: "UUID of the Kubernetes cluster.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"slug": schema.StringAttribute{
				Description: "Addon slug identifier (e.g., cert-manager, ingress-nginx).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"values": schema.StringAttribute{
				Description: "Custom Helm values as JSON string.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Addon display name.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Addon description.",
				Computed:    true,
			},
			"category": schema.StringAttribute{
				Description: "Addon category.",
				Computed:    true,
			},
			"namespace": schema.StringAttribute{
				Description: "Kubernetes namespace where the addon is installed.",
				Computed:    true,
			},
			"helm_chart": schema.StringAttribute{
				Description: "Helm chart name.",
				Computed:    true,
			},
			"status": schema.StringAttribute{
				Description: "Current installation status.",
				Computed:    true,
			},
			"version": schema.StringAttribute{
				Description: "Installed version.",
				Computed:    true,
			},
			"installed_at": schema.StringAttribute{
				Description: "When the addon was installed.",
				Computed:    true,
			},
		},
	}
}

func (r *kubernetesAddonResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	r.client = client
}

func (r *kubernetesAddonResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesAddonResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var values json.RawMessage
	if !plan.Values.IsNull() && !plan.Values.IsUnknown() {
		valuesStr := plan.Values.ValueString()
		if !json.Valid([]byte(valuesStr)) {
			resp.Diagnostics.AddError("Invalid JSON", "The 'values' attribute must be valid JSON.")
			return
		}
		values = json.RawMessage(valuesStr)
	}

	// Retry install in case cluster has active tasks (HTTP 409)
	var installed *client.KubernetesInstalledAddon
	var err error
	for attempt := 0; attempt < 30; attempt++ {
		installed, err = r.client.Kubernetes.InstallAddon(ctx, plan.ClusterID.ValueString(), plan.Slug.ValueString(), values)
		if err == nil {
			break
		}
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsConflict() {
			time.Sleep(15 * time.Second)
			continue
		}
		break
	}
	if err != nil {
		resp.Diagnostics.AddError("Error installing addon", err.Error())
		return
	}

	// Re-read from installed addons list to get full details
	addons, err := r.client.Kubernetes.ListInstalledAddons(ctx, plan.ClusterID.ValueString())
	if err == nil {
		for _, a := range addons {
			if a.UUID == installed.UUID {
				installed = &a
				break
			}
		}
	}

	r.mapAddonToState(&plan, installed)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *kubernetesAddonResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state kubernetesAddonResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	installed, err := r.client.Kubernetes.ListInstalledAddons(ctx, state.ClusterID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading installed addons", err.Error())
		return
	}

	// Find the addon by UUID
	var found *client.KubernetesInstalledAddon
	for _, a := range installed {
		if a.UUID == state.ID.ValueString() {
			found = &a
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.mapAddonToState(&state, found)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *kubernetesAddonResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// No update supported — all mutable fields use RequiresReplace
	resp.Diagnostics.AddError("Update not supported",
		"Kubernetes addons cannot be updated in place. Changes to slug or values require replacement.")
}

func (r *kubernetesAddonResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state kubernetesAddonResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retry delete in case there are active tasks (HTTP 409)
	var err error
	for attempt := 0; attempt < 30; attempt++ {
		err = r.client.Kubernetes.UninstallAddon(ctx, state.ClusterID.ValueString(), state.ID.ValueString())
		if err == nil {
			return
		}
		if apiErr, ok := err.(*client.APIError); ok {
			if apiErr.IsNotFound() {
				return
			}
			if apiErr.IsConflict() {
				time.Sleep(15 * time.Second)
				continue
			}
		}
		break
	}
	if err != nil {
		resp.Diagnostics.AddError("Error uninstalling addon", err.Error())
		return
	}
}

func (r *kubernetesAddonResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID",
			"Expected format: cluster_uuid/addon_uuid")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (r *kubernetesAddonResource) mapAddonToState(state *kubernetesAddonResourceModel, addon *client.KubernetesInstalledAddon) {
	state.ID = types.StringValue(addon.UUID)
	state.Name = types.StringValue(addon.Addon.Name)
	state.Slug = types.StringValue(addon.Addon.Slug)
	state.Description = types.StringValue(addon.Addon.Description)
	state.Category = types.StringValue(addon.Addon.Category)
	state.Namespace = types.StringValue(addon.Addon.Namespace)
	state.HelmChart = types.StringValue(addon.Addon.HelmChart)
	state.Status = types.StringValue(addon.Status)
	state.Version = types.StringValue(addon.InstalledVersion)
	state.InstalledAt = types.StringValue(addon.InstalledAt)
}
