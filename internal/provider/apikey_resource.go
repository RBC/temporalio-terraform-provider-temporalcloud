package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type (
	apiKeyResource struct {
		client *client.Client
	}

	apiKeyResourceModel struct {
		ID          types.String   `tfsdk:"id"`
		State       types.String   `tfsdk:"state"`
		OwnerType   types.String   `tfsdk:"owner_type"`
		OwnerID     types.String   `tfsdk:"owner_id"`
		DisplayName types.String   `tfsdk:"display_name"`
		Token       types.String   `tfsdk:"token"`
		Description types.String   `tfsdk:"description"`
		ExpiryTime  types.String   `tfsdk:"expiry_time"` // ISO 8601 format
		Disabled    types.Bool     `tfsdk:"disabled"`
		Timeouts    timeouts.Value `tfsdk:"timeouts"`
	}
)

var (
	_ resource.Resource              = (*apiKeyResource)(nil)
	_ resource.ResourceWithConfigure = (*apiKeyResource)(nil)
)

func NewApiKeyResource() resource.Resource {
	return &apiKeyResource{}
}

func (r *apiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = client
}

func (r *apiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_apikey"
}

func (r *apiKeyResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provisions a Temporal Cloud API key.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the API key.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Description: "The current state of the API key.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"owner_type": schema.StringAttribute{
				Description: "The type of the owner to create the API key.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"owner_id": schema.StringAttribute{
				Description: "The ID of the owner to create the API key for.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				Description: "The display name for the API key.",
				Required:    true,
			},
			"token": schema.StringAttribute{
				Description: "The token for the API key. This field is populated with the full key when creating an API key. To retrieve the value of this field, use an output.tf file and follow Terraform's guidance on working with sensitive fields.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				Description: "The description for the API key.",
				Optional:    true,
			},
			"expiry_time": schema.StringAttribute{
				Description: "The expiry time for the API key in ISO 8601 format.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"disabled": schema.BoolAttribute{
				Description: "Whether the API key is disabled.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Delete: true,
			}),
		},
	}
}

func (r *apiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, defaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Parse the expiry time from the plan
	expiryTimeString := plan.ExpiryTime.ValueString()
	expiryTime, err := time.Parse(time.RFC3339, expiryTimeString)
	if err != nil {
		resp.Diagnostics.AddError("Invalid ExpiryTime", "Could not parse ExpiryTime from plan: "+err.Error())
		return
	}

	// Convert time.Time to protobuf Timestamp
	expiryTimestamp := timestamppb.New(expiryTime)

	description := ""
	if !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}

	disabled := false
	if !plan.Disabled.IsNull() {
		disabled = plan.Disabled.ValueBool()
	}

	ownerType, err := enums.ToOwnerType(plan.OwnerType.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(err.Error(), "")
		return
	}
	svcResp, err := r.client.CloudService().CreateApiKey(ctx, &cloudservicev1.CreateApiKeyRequest{
		Spec: &identityv1.ApiKeySpec{
			OwnerId:     plan.OwnerID.ValueString(),
			OwnerType:   ownerType,
			DisplayName: plan.DisplayName.ValueString(),
			Description: description,
			ExpiryTime:  expiryTimestamp,
			Disabled:    disabled,
		},
		AsyncOperationId: uuid.New().String(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to create API key", err.Error())
		return
	}
	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to create API key", err.Error())
		return
	}

	apiKey, err := r.client.CloudService().GetApiKey(ctx, &cloudservicev1.GetApiKeyRequest{
		KeyId: svcResp.GetKeyId(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get API key after creation", err.Error())
		return
	}

	err = updateApiKeyModelFromSpec(&plan, apiKey.ApiKey)
	if err != nil {
		resp.Diagnostics.AddError("Failed to convert apikey spec", err.Error())
		return
	}
	plan.Token = types.StringValue(svcResp.Token)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey, err := r.client.CloudService().GetApiKey(ctx, &cloudservicev1.GetApiKeyRequest{
		KeyId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "API Key Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to get API key", err.Error())
		return
	}

	if err := updateApiKeyModelFromSpec(&state, apiKey.ApiKey); err != nil {
		resp.Diagnostics.AddError("Failed to convert apikey spec", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *apiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey, err := r.client.CloudService().GetApiKey(ctx, &cloudservicev1.GetApiKeyRequest{
		KeyId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current API key status", err.Error())
		return
	}

	// Parse the expiry time from the plan
	expiryTimeString := plan.ExpiryTime.ValueString()
	expiryTime, err := time.Parse(time.RFC3339, expiryTimeString)
	if err != nil {
		resp.Diagnostics.AddError("Invalid ExpiryTime", "Could not parse ExpiryTime from plan: "+err.Error())
		return
	}

	// Convert time.Time to protobuf Timestamp
	expiryTimestamp := timestamppb.New(expiryTime)

	description := ""
	if !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}

	disabled := false
	if !plan.Disabled.IsNull() {
		disabled = plan.Disabled.ValueBool()
	}

	ownerType, err := enums.ToOwnerType(plan.OwnerType.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(err.Error(), "")
		return
	}
	svcResp, err := r.client.CloudService().UpdateApiKey(ctx, &cloudservicev1.UpdateApiKeyRequest{
		KeyId: plan.ID.ValueString(),
		Spec: &identityv1.ApiKeySpec{
			OwnerId:     plan.OwnerID.ValueString(),
			OwnerType:   ownerType,
			DisplayName: plan.DisplayName.ValueString(),
			Description: description,
			ExpiryTime:  expiryTimestamp,
			Disabled:    disabled,
		},
		ResourceVersion:  apiKey.GetApiKey().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update API key", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to update API key", err.Error())
		return
	}

	apiKey, err = r.client.CloudService().GetApiKey(ctx, &cloudservicev1.GetApiKeyRequest{
		KeyId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get API key after update", err.Error())
		return
	}

	if err := updateApiKeyModelFromSpec(&plan, apiKey.ApiKey); err != nil {
		resp.Diagnostics.AddError("Failed to convert apikey spec", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *apiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey, err := r.client.CloudService().GetApiKey(ctx, &cloudservicev1.GetApiKeyRequest{
		KeyId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "API Key Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to get current API key status", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	svcResp, err := r.client.CloudService().DeleteApiKey(ctx, &cloudservicev1.DeleteApiKeyRequest{
		KeyId:            state.ID.ValueString(),
		ResourceVersion:  apiKey.GetApiKey().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "API Key Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to delete API key", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to delete API key", err.Error())
	}
}

func updateApiKeyModelFromSpec(state *apiKeyResourceModel, apikey *identityv1.ApiKey) error {
	state.ID = types.StringValue(apikey.GetId())
	stateStr, err := enums.FromResourceState(apikey.GetState())
	if err != nil {
		return err
	}
	state.State = types.StringValue(stateStr)
	state.OwnerID = types.StringValue(apikey.GetSpec().GetOwnerId())
	ownerType, err := enums.FromOwnerType(apikey.GetSpec().GetOwnerType())
	if err != nil {
		return err
	}
	state.OwnerType = types.StringValue(ownerType)
	state.DisplayName = types.StringValue(apikey.GetSpec().GetDisplayName())
	if apikey.GetSpec().GetDescription() != "" {
		state.Description = types.StringValue(apikey.GetSpec().GetDescription())
	}
	state.ExpiryTime = types.StringValue(apikey.GetSpec().GetExpiryTime().AsTime().Format(time.RFC3339))
	state.Disabled = types.BoolValue(apikey.GetSpec().GetDisabled())

	return nil
}
