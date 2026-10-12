package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/quixgit/greenops-reliabilix/terraform-provider-reliabilix/internal/client"
)

type projectModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	FunctionalUnit types.String `tfsdk:"functional_unit"`
}

type projectResource struct{ c *client.Client }

func newProjectResource() resource.Resource { return &projectResource{} }

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A project: the unit costs and carbon are reported for. The API has no update, so changing the name or the unit replaces it; a project with cloud connections or data cannot be deleted.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":            schema.StringAttribute{Required: true, Description: "1 to 120 characters, unique in the organization.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"functional_unit": schema.StringAttribute{Required: true, Description: "What one unit of useful work is (the SCI denominator), for example api_request.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		},
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.c.CreateProject(ctx, plan.Name.ValueString(), plan.FunctionalUnit.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Creating the project failed", err.Error())
		return
	}
	plan.ID = types.StringValue(p.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var st projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &st)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.c.GetProject(ctx, st.ID.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading the project failed", err.Error())
		return
	}
	st.Name, st.FunctionalUnit = types.StringValue(p.Name), types.StringValue(p.FunctionalUnit)
	resp.Diagnostics.Append(resp.State.Set(ctx, st)...)
}

// Update is never reached: every attribute replaces the project.
func (r *projectResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var st projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &st)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.DeleteProject(ctx, st.ID.ValueString()); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("Deleting the project failed", err.Error())
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ---- data source: find a project by name

type projectDataSource struct{ c *client.Client }

func newProjectDataSource() datasource.DataSource { return &projectDataSource{} }

func (d *projectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *projectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "An existing project, found by name (for example one made in the console).",
		Attributes: map[string]dschema.Attribute{
			"name":            dschema.StringAttribute{Required: true},
			"id":              dschema.StringAttribute{Computed: true},
			"functional_unit": dschema.StringAttribute{Computed: true},
		},
	}
}

func (d *projectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.c = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m projectModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ps, err := d.c.ListProjects(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Listing projects failed", err.Error())
		return
	}
	for _, p := range ps {
		if p.Name == m.Name.ValueString() {
			m.ID, m.FunctionalUnit = types.StringValue(p.ID), types.StringValue(p.FunctionalUnit)
			resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
			return
		}
	}
	resp.Diagnostics.AddError("Project not found", "no project is named "+m.Name.ValueString())
}
