package provider

import (
	"context"
	"fmt"
	"net"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"golang.org/x/crypto/ssh"
)

// hostnameValidator validates that a string is a valid RFC 1123 hostname
type hostnameValidator struct{}

func (v hostnameValidator) Description(_ context.Context) string {
	return "value must be a valid RFC 1123 hostname"
}

func (v hostnameValidator) MarkdownDescription(_ context.Context) string {
	return "value must be a valid RFC 1123 hostname"
}

func (v hostnameValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	hostname := req.ConfigValue.ValueString()

	// RFC 1123 hostname validation
	if len(hostname) > 253 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Hostname",
			"Hostname must be 253 characters or less",
		)
		return
	}

	// Hostname regex: lowercase alphanumeric and hyphens, must start and end with alphanumeric
	hostnameRegex := regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?$`)
	if !hostnameRegex.MatchString(hostname) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Hostname",
			"Hostname must consist of lowercase alphanumeric characters or '-', and must start and end with an alphanumeric character",
		)
	}
}

// HostnameValidator returns a validator for RFC 1123 hostnames
func HostnameValidator() validator.String {
	return hostnameValidator{}
}

// cidrValidator validates that a string is a valid CIDR and optionally in a private range
type cidrValidator struct {
	requirePrivate bool
}

func (v cidrValidator) Description(_ context.Context) string {
	if v.requirePrivate {
		return "value must be a valid private CIDR (10.0.0.0/8, 172.16.0.0/12, or 192.168.0.0/16)"
	}
	return "value must be a valid CIDR notation"
}

func (v cidrValidator) MarkdownDescription(_ context.Context) string {
	return v.Description(nil)
}

func (v cidrValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	cidr := req.ConfigValue.ValueString()

	// Parse CIDR
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid CIDR",
			fmt.Sprintf("Must be valid CIDR notation: %s", err),
		)
		return
	}

	// Check if private IP range (if required)
	if v.requirePrivate {
		privateRanges := []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
		}

		isPrivate := false
		for _, pr := range privateRanges {
			_, privateNet, _ := net.ParseCIDR(pr)
			if privateNet.Contains(ipNet.IP) {
				isPrivate = true
				break
			}
		}

		if !isPrivate {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid CIDR",
				"Must be a private IP range (10.0.0.0/8, 172.16.0.0/12, or 192.168.0.0/16)",
			)
		}
	}
}

// CIDRValidator returns a validator for CIDR notation
func CIDRValidator(requirePrivate bool) validator.String {
	return cidrValidator{requirePrivate: requirePrivate}
}

// sshKeyValidator validates that a string is a valid SSH public key
type sshKeyValidator struct{}

func (v sshKeyValidator) Description(_ context.Context) string {
	return "value must be a valid SSH public key"
}

func (v sshKeyValidator) MarkdownDescription(_ context.Context) string {
	return "value must be a valid SSH public key"
}

func (v sshKeyValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	publicKey := req.ConfigValue.ValueString()

	// Parse SSH public key
	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey))
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid SSH Public Key",
			fmt.Sprintf("Must be a valid SSH public key: %s", err),
		)
	}
}

// SSHKeyValidator returns a validator for SSH public keys
func SSHKeyValidator() validator.String {
	return sshKeyValidator{}
}

// strongPasswordValidator validates password strength
type strongPasswordValidator struct{}

func (v strongPasswordValidator) Description(_ context.Context) string {
	return "value must be at least 8 characters with uppercase, lowercase, and number"
}

func (v strongPasswordValidator) MarkdownDescription(_ context.Context) string {
	return "value must be at least 8 characters with uppercase, lowercase, and number"
}

func (v strongPasswordValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	password := req.ConfigValue.ValueString()

	if len(password) < 8 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Weak Password",
			"Password must be at least 8 characters",
		)
		return
	}

	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)

	if !hasUpper || !hasLower || !hasNumber {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Weak Password",
			"Password must contain at least one uppercase letter, one lowercase letter, and one number",
		)
	}
}

// StrongPasswordValidator returns a validator for strong passwords
func StrongPasswordValidator() validator.String {
	return strongPasswordValidator{}
}
