package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// isNotFound reports whether err is (or wraps) an API 404.
func isNotFound(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && apiErr.IsNotFound()
}

// isConflict reports whether err is (or wraps) an API 409.
func isConflict(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && apiErr.IsConflict()
}

// retryWhile calls fn until it succeeds, retry(err) is false, or timeout passes. It is
// used for API calls that answer 409 while another operation on the resource runs.
func retryWhile(ctx context.Context, timeout, interval time.Duration, retry func(error) bool, fn func() error) error {
	deadline := time.Now().Add(timeout)
	for {
		err := fn()
		if err == nil || !retry(err) || time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// knownInt returns the int value of an attribute, or nil when it is null or unknown.
func knownInt(v types.Int64) *int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := int(v.ValueInt64())
	return &i
}

// knownString returns the value of an attribute, or nil when it is null or unknown.
func knownString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// stringOrNull maps nil and "" to null.
func stringOrNull(s *string) types.String {
	if s == nil || *s == "" {
		return types.StringNull()
	}
	return types.StringValue(*s)
}

// stringsFromSet reads a set of strings; null and unknown give nil.
func stringsFromSet(ctx context.Context, v types.Set, diags *diag.Diagnostics) []string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(v.ElementsAs(ctx, &out, false)...)
	return out
}

// intsFromSet reads a set of numbers; null and unknown give nil.
func intsFromSet(ctx context.Context, v types.Set, diags *diag.Diagnostics) []int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	var out []int64
	diags.Append(v.ElementsAs(ctx, &out, false)...)
	return out
}

// diffInts returns the values of want missing from have, and of have missing from want.
func diffInts(have, want []int64) (add, remove []int64) {
	h := map[int64]bool{}
	for _, v := range have {
		h[v] = true
	}
	w := map[int64]bool{}
	for _, v := range want {
		w[v] = true
		if !h[v] {
			add = append(add, v)
		}
	}
	for _, v := range have {
		if !w[v] {
			remove = append(remove, v)
		}
	}
	return add, remove
}

// diffStrings returns the values of want missing from have, and of have missing from want.
func diffStrings(have, want []string) (add, remove []string) {
	h := map[string]bool{}
	for _, v := range have {
		h[v] = true
	}
	w := map[string]bool{}
	for _, v := range want {
		w[v] = true
		if !h[v] {
			add = append(add, v)
		}
	}
	for _, v := range have {
		if !w[v] {
			remove = append(remove, v)
		}
	}
	return add, remove
}

// StringOneOf accepts only the given values (see stringOneOfValidator).
func StringOneOf(values ...string) validator.String {
	return stringOneOfValidator{valid: values}
}

type int64BetweenValidator struct {
	min, max int64
}

func (v int64BetweenValidator) Description(_ context.Context) string {
	return fmt.Sprintf("value must be between %d and %d", v.min, v.max)
}

func (v int64BetweenValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v int64BetweenValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	got := req.ConfigValue.ValueInt64()
	if got < v.min || got > v.max {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid value",
			fmt.Sprintf("%d is not valid: %s.", got, v.Description(ctx)))
	}
}

// Int64Between accepts values from min to max, both included.
func Int64Between(min, max int64) validator.Int64 {
	return int64BetweenValidator{min: min, max: max}
}

// knownBool returns the value of an attribute, or nil when it is null or unknown.
func knownBool(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

// requiresReplaceIfRemovedInt64 forces replacement when a value set before is removed from
// the configuration (for attributes the API cannot clear).
func requiresReplaceIfRemovedInt64(_ context.Context, req planmodifier.Int64Request, resp *int64planmodifier.RequiresReplaceIfFuncResponse) {
	resp.RequiresReplace = req.ConfigValue.IsNull() && !req.StateValue.IsNull()
}
