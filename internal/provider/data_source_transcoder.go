package provider

import (
	"context"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Transcoding jobs are one-shot work items, not infrastructure: the provider only reads them.

var (
	_ datasource.DataSource              = &transcoderJobDataSource{}
	_ datasource.DataSourceWithConfigure = &transcoderJobDataSource{}
	_ datasource.DataSource              = &transcoderJobsDataSource{}
	_ datasource.DataSourceWithConfigure = &transcoderJobsDataSource{}
)

type transcoderJobModel struct {
	ID                types.String              `tfsdk:"id"`
	Status            types.String              `tfsdk:"status"`
	Progress          types.Int64               `tfsdk:"progress"`
	TotalSegments     types.Int64               `tfsdk:"total_segments"`
	CompletedSegments types.Int64               `tfsdk:"completed_segments"`
	BatchID           types.String              `tfsdk:"batch_id"`
	Error             types.String              `tfsdk:"error"`
	InputURL          types.String              `tfsdk:"input_url"`
	OutputBucket      types.String              `tfsdk:"output_bucket"`
	OutputPath        types.String              `tfsdk:"output_path"`
	Outputs           []transcoderArtifactModel `tfsdk:"outputs"`
	CreatedAt         types.String              `tfsdk:"created_at"`
}

type transcoderArtifactModel struct {
	Type   types.String `tfsdk:"type"`
	Bucket types.String `tfsdk:"bucket"`
	Key    types.String `tfsdk:"key"`
}

func transcoderJobAttributes(idRequired bool) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Description: "UUID of the job.",
			Required:    idRequired,
			Computed:    !idRequired,
		},
		"status": schema.StringAttribute{
			Description: "queued, analyzing, encoding, finalizing, completed, failed or canceled.",
			Computed:    true,
		},
		"progress":           schema.Int64Attribute{Description: "Progress, 0 to 100.", Computed: true},
		"total_segments":     schema.Int64Attribute{Description: "Segments the video was split into.", Computed: true},
		"completed_segments": schema.Int64Attribute{Description: "Segments already encoded.", Computed: true},
		"batch_id":           schema.StringAttribute{Description: "Batch the job was submitted in, if any.", Computed: true},
		"error":              schema.StringAttribute{Description: "Error of a failed job.", Computed: true},
		"input_url":          schema.StringAttribute{Description: "Input URL, for jobs read from a URL.", Computed: true},
		"output_bucket":      schema.StringAttribute{Description: "Destination bucket.", Computed: true},
		"output_path":        schema.StringAttribute{Description: "Destination prefix in the bucket.", Computed: true},
		"outputs": schema.ListNestedAttribute{
			Description: "Files produced by a finished job.",
			Computed:    true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"type":   schema.StringAttribute{Description: "Output type: file, hls, thumbnails or gif.", Computed: true},
					"bucket": schema.StringAttribute{Description: "Bucket.", Computed: true},
					"key":    schema.StringAttribute{Description: "Object key.", Computed: true},
				},
			},
		},
		"created_at": schema.StringAttribute{Description: "Creation time.", Computed: true},
	}
}

func mapTranscoderJob(job *client.TranscoderJob) transcoderJobModel {
	m := transcoderJobModel{
		ID:                types.StringValue(job.UUID),
		Status:            types.StringValue(job.Status),
		Progress:          types.Int64Value(int64(job.Progress)),
		TotalSegments:     types.Int64Value(int64(job.TotalSegments)),
		CompletedSegments: types.Int64Value(int64(job.CompletedSegments)),
		BatchID:           stringOrNull(job.BatchID),
		Error:             stringOrNull(job.Error),
		InputURL:          types.StringNull(),
		OutputBucket:      types.StringNull(),
		OutputPath:        types.StringNull(),
		Outputs:           []transcoderArtifactModel{},
		CreatedAt:         types.StringValue(job.CreatedAt),
	}
	if job.Input != nil {
		m.InputURL = stringOrNull(job.Input.URL)
	}
	if job.Output != nil && job.Output.S3 != nil {
		m.OutputBucket = types.StringValue(job.Output.S3.Bucket)
		m.OutputPath = types.StringValue(job.Output.S3.Path)
	}
	for _, o := range job.Outputs {
		m.Outputs = append(m.Outputs, transcoderArtifactModel{
			Type:   types.StringValue(o.Type),
			Bucket: types.StringValue(o.Bucket),
			Key:    types.StringValue(o.Key),
		})
	}
	return m
}

// ---- cubepath_transcoder_job ----

func NewTranscoderJobDataSource() datasource.DataSource {
	return &transcoderJobDataSource{}
}

type transcoderJobDataSource struct {
	client *client.Client
}

func (d *transcoderJobDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_transcoder_job"
}

func (d *transcoderJobDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a Video Transcoder job: status, progress and the files it produced. Jobs are submitted " +
			"with the API, the CLI or the SDKs; Terraform only reads them.",
		Attributes: transcoderJobAttributes(true),
	}
}

func (d *transcoderJobDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureDataSourceClient(req.ProviderData, resp); c != nil {
		d.client = c
	}
}

func (d *transcoderJobDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	job, err := d.client.Transcoder.GetJob(ctx, id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading transcoding job", err.Error())
		return
	}
	state := mapTranscoderJob(job)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ---- cubepath_transcoder_jobs ----

func NewTranscoderJobsDataSource() datasource.DataSource {
	return &transcoderJobsDataSource{}
}

type transcoderJobsDataSource struct {
	client *client.Client
}

type transcoderJobsModel struct {
	ID      types.String         `tfsdk:"id"`
	BatchID types.String         `tfsdk:"batch_id"`
	Limit   types.Int64          `tfsdk:"limit"`
	Jobs    []transcoderJobModel `tfsdk:"jobs"`
}

func (d *transcoderJobsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_transcoder_jobs"
}

func (d *transcoderJobsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Video Transcoder jobs, newest first.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Placeholder identifier.", Computed: true},
			"batch_id": schema.StringAttribute{
				Description: "Only list the jobs of this batch.",
				Optional:    true,
			},
			"limit": schema.Int64Attribute{
				Description: "Maximum number of jobs, 1 to 500. Defaults to 100.",
				Optional:    true,
			},
			"jobs": schema.ListNestedAttribute{
				Description:  "Jobs.",
				Computed:     true,
				NestedObject: schema.NestedAttributeObject{Attributes: transcoderJobAttributes(false)},
			},
		},
	}
}

func (d *transcoderJobsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureDataSourceClient(req.ProviderData, resp); c != nil {
		d.client = c
	}
}

func (d *transcoderJobsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state transcoderJobsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	limit := 100
	if !state.Limit.IsNull() {
		limit = int(state.Limit.ValueInt64())
	}
	if limit < 1 || limit > 500 {
		resp.Diagnostics.AddAttributeError(path.Root("limit"), "Invalid limit", "limit must be between 1 and 500.")
		return
	}
	jobs, err := d.client.Transcoder.ListJobs(ctx, state.BatchID.ValueString(), limit, 0)
	if err != nil {
		resp.Diagnostics.AddError("Error reading transcoding jobs", err.Error())
		return
	}
	state.ID = types.StringValue("transcoder_jobs")
	state.Jobs = make([]transcoderJobModel, 0, len(jobs))
	for i := range jobs {
		state.Jobs = append(state.Jobs, mapTranscoderJob(&jobs[i]))
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
