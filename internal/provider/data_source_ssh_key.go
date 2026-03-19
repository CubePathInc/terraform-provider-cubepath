package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/cubepath/terraform-provider-cubepath/internal/client"
)

var (
	_ datasource.DataSource              = &SSHKeyDataSource{}
	_ datasource.DataSourceWithConfigure = &SSHKeyDataSource{}
)

func NewSSHKeyDataSource() datasource.DataSource {
	return &SSHKeyDataSource{}
}

type SSHKeyDataSource struct {
	client *client.Client
}

type SSHKeyDataSourceModel struct {
	Name        types.String `tfsdk:"name"`
	ID          types.String `tfsdk:"id"`
	Fingerprint types.String `tfsdk:"fingerprint"`
	KeyType     types.String `tfsdk:"key_type"`
}

func (d *SSHKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_key"
}

func (d *SSHKeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches an existing SSH key by name",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "SSH key name to lookup",
				Required:    true,
			},
			"id": schema.StringAttribute{
				Description: "SSH key ID",
				Computed:    true,
			},
			"fingerprint": schema.StringAttribute{
				Description: "SSH key fingerprint",
				Computed:    true,
			},
			"key_type": schema.StringAttribute{
				Description: "SSH key type (e.g., ssh-rsa, ssh-ed25519)",
				Computed:    true,
			},
		},
	}
}

func (d *SSHKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *SSHKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state SSHKeyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get all SSH keys
	sshKeys, err := d.client.SSHKeys.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read SSH Keys",
			err.Error(),
		)
		return
	}

	// Find the SSH key with matching name
	var found *client.SSHKey
	for _, key := range sshKeys {
		if key.Name == state.Name.ValueString() {
			found = &key
			break
		}
	}

	if found == nil {
		resp.Diagnostics.AddError(
			"SSH Key Not Found",
			fmt.Sprintf("No SSH key found with name: %s", state.Name.ValueString()),
		)
		return
	}

	// Map response to state
	state.ID = types.StringValue(fmt.Sprintf("%d", found.ID))
	state.Fingerprint = types.StringValue(found.Fingerprint)
	state.KeyType = types.StringValue(found.KeyType)

	// Set state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
