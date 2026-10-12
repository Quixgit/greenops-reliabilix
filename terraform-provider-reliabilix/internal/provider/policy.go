package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/quixgit/greenops-reliabilix/terraform-provider-reliabilix/internal/client"
)

type policyModel struct {
	ProjectID            types.String  `tfsdk:"project_id"`
	AllowedRegions       types.Set     `tfsdk:"allowed_regions"`
	CICarbonWarnKg       types.Float64 `tfsdk:"ci_carbon_warn_kg"`
	CICarbonBlockKg      types.Float64 `tfsdk:"ci_carbon_block_kg"`
	CICostWarn           types.Float64 `tfsdk:"ci_cost_warn"`
	CICostBlock          types.Float64 `tfsdk:"ci_cost_block"`
	AlertCarbonMonthlyKg types.Float64 `tfsdk:"alert_carbon_monthly_kg"`
}

type policyResource struct{ c *client.Client }

func newPolicyResource() resource.Resource { return &policyResource{} }

func (r *policyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_policy"
}

func (r *policyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	opt := func(desc string) schema.Float64Attribute { return schema.Float64Attribute{Optional: true, Description: desc} }
	resp.Schema = schema.Schema{
		Description: "The data-residency allow-list and thresholds of a project. Which recommendation types may be applied automatically is not managed here and is never changed by this resource.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"allowed_regions": schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: false,
				Description: "Regions workloads may move to. While empty, residency is unknown and no region move is ever recommended."},
			"ci_carbon_warn_kg":       opt("CI gate: warn above this monthly carbon increase (kg CO2e)."),
			"ci_carbon_block_kg":      opt("CI gate: block above this monthly carbon increase (kg CO2e)."),
			"ci_cost_warn":            opt("CI gate: warn above this monthly cost increase."),
			"ci_cost_block":           opt("CI gate: block above this monthly cost increase."),
			"alert_carbon_monthly_kg": opt("Open a carbon alert when the project's emissions in a month approach this (kg CO2e)."),
		},
	}
}

func (r *policyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func f64(v types.Float64) *float64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	x := v.ValueFloat64()
	return &x
}

func tf64(v *float64) types.Float64 {
	if v == nil {
		return types.Float64Null()
	}
	return types.Float64Value(*v)
}

func (m policyModel) toAPI(ctx context.Context) (client.Policy, error) {
	p := client.Policy{CICarbonWarnKg: f64(m.CICarbonWarnKg), CICarbonBlockKg: f64(m.CICarbonBlockKg), CICostWarn: f64(m.CICostWarn), CICostBlock: f64(m.CICostBlock), AlertCarbonMonthlyKg: f64(m.AlertCarbonMonthlyKg)}
	if !m.AllowedRegions.IsNull() && !m.AllowedRegions.IsUnknown() {
		var regions []string
		if d := m.AllowedRegions.ElementsAs(ctx, &regions, false); d.HasError() {
			return p, errors.New("allowed_regions must be a set of strings")
		}
		p.AllowedRegions = regions
	}
	return p, nil
}

// fromAPI keeps an unset allowed_regions null when the server has none, so an omitted attribute is not reported as drift.
func (m *policyModel) fromAPI(p client.Policy) {
	if len(p.AllowedRegions) == 0 && m.AllowedRegions.IsNull() {
		// stay null
	} else {
		els := make([]attr.Value, 0, len(p.AllowedRegions))
		for _, r := range p.AllowedRegions {
			els = append(els, types.StringValue(r))
		}
		m.AllowedRegions = types.SetValueMust(types.StringType, els)
	}
	m.CICarbonWarnKg, m.CICarbonBlockKg, m.CICostWarn, m.CICostBlock, m.AlertCarbonMonthlyKg =
		tf64(p.CICarbonWarnKg), tf64(p.CICarbonBlockKg), tf64(p.CICostWarn), tf64(p.CICostBlock), tf64(p.AlertCarbonMonthlyKg)
}

func (r *policyResource) apply(ctx context.Context, m *policyModel) error {
	p, err := m.toAPI(ctx)
	if err != nil {
		return err
	}
	out, err := r.c.PutPolicy(ctx, m.ProjectID.ValueString(), p)
	if err != nil {
		return err
	}
	m.fromAPI(out)
	return nil
}

func (r *policyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m policyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.apply(ctx, &m); err != nil {
		resp.Diagnostics.AddError("Writing the policy failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *policyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m policyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.c.GetPolicy(ctx, m.ProjectID.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading the policy failed", err.Error())
		return
	}
	m.fromAPI(p)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *policyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m policyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.apply(ctx, &m); err != nil {
		resp.Diagnostics.AddError("Writing the policy failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

// Delete clears what this resource manages; the project and the automatic-application list stay as they are.
func (r *policyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m policyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.c.PutPolicy(ctx, m.ProjectID.ValueString(), client.Policy{}); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("Clearing the policy failed", err.Error())
	}
}

func (r *policyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("project_id"), req, resp)
}
