package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/quixgit/greenops-reliabilix/terraform-provider-reliabilix/internal/client"
)

type budgetModel struct {
	ID        types.String  `tfsdk:"id"`
	ProjectID types.String  `tfsdk:"project_id"`
	Amount    types.Float64 `tfsdk:"amount"`
	Currency  types.String  `tfsdk:"currency"`
	Period    types.String  `tfsdk:"period"`
}

type budgetResource struct{ c *client.Client }

func newBudgetResource() resource.Resource { return &budgetResource{} }

func (r *budgetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_budget"
}

func (r *budgetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A budget for the whole organization or one project. The API has no update, so any change replaces it.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"project_id": schema.StringAttribute{Optional: true, Description: "Leave out for an organization-wide budget.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"amount":     schema.Float64Attribute{Required: true, PlanModifiers: []planmodifier.Float64{float64planmodifier.RequiresReplace()}},
			"currency":   schema.StringAttribute{Required: true, Description: "Three letters, like USD.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"period":     schema.StringAttribute{Required: true, Description: "monthly, quarterly or yearly.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		},
	}
}

func (r *budgetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *budgetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m budgetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	b := client.Budget{Amount: m.Amount.ValueFloat64(), Currency: m.Currency.ValueString(), Period: m.Period.ValueString()}
	if !m.ProjectID.IsNull() && !m.ProjectID.IsUnknown() {
		id := m.ProjectID.ValueString()
		b.ProjectID = &id
	}
	out, err := r.c.CreateBudget(ctx, b)
	if err != nil {
		resp.Diagnostics.AddError("Creating the budget failed", err.Error())
		return
	}
	m.ID = types.StringValue(out.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *budgetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m budgetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	b, err := r.c.GetBudget(ctx, m.ID.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading the budget failed", err.Error())
		return
	}
	m.Amount, m.Currency, m.Period = types.Float64Value(b.Amount), types.StringValue(b.Currency), types.StringValue(b.Period)
	m.ProjectID = types.StringPointerValue(b.ProjectID)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

// Update is never reached: every attribute replaces the budget.
func (r *budgetResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}

func (r *budgetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m budgetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.DeleteBudget(ctx, m.ID.ValueString()); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("Deleting the budget failed", err.Error())
	}
}

func (r *budgetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
