package client

import (
	"context"
	"fmt"
)

// SSHKeys provides methods for managing SSH keys
type SSHKeys struct {
	client *Client
}

// NewSSHKeys creates a new SSH keys service
func NewSSHKeys(client *Client) *SSHKeys {
	return &SSHKeys{client: client}
}

// sshKeyCreateResponse represents the API response when creating an SSH key
type sshKeyCreateResponse struct {
	Detail      string `json:"detail"`
	SSHKeyID    int    `json:"ssh_key_id"`
	Name        string `json:"name"`
	KeyType     string `json:"key_type"`
	Fingerprint string `json:"fingerprint"`
}

// Create creates a new SSH key
func (s *SSHKeys) Create(ctx context.Context, req *CreateSSHKeyRequest) (*SSHKey, error) {
	var result sshKeyCreateResponse
	err := s.client.Post(ctx, "/sshkey/create", req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH key: %w", err)
	}
	return &SSHKey{
		ID:          result.SSHKeyID,
		Name:        result.Name,
		Fingerprint: result.Fingerprint,
		KeyType:     result.KeyType,
	}, nil
}

// sshKeysResponse wraps the API response for listing SSH keys
type sshKeysResponse struct {
	SSHKeys []SSHKey `json:"sshkeys"`
}

// List retrieves all SSH keys for the user
func (s *SSHKeys) List(ctx context.Context) ([]SSHKey, error) {
	var result sshKeysResponse
	err := s.client.Get(ctx, "/sshkey/user/sshkeys", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list SSH keys: %w", err)
	}
	return result.SSHKeys, nil
}

// Get retrieves a specific SSH key by ID
func (s *SSHKeys) Get(ctx context.Context, id int) (*SSHKey, error) {
	keys, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		if key.ID == id {
			return &key, nil
		}
	}

	return nil, &APIError{
		StatusCode: 404,
		Message:    "Not Found",
		Detail:     fmt.Sprintf("SSH key with ID %d not found", id),
	}
}

// GetByName retrieves a specific SSH key by name
func (s *SSHKeys) GetByName(ctx context.Context, name string) (*SSHKey, error) {
	keys, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		if key.Name == name {
			return &key, nil
		}
	}

	return nil, &APIError{
		StatusCode: 404,
		Message:    "Not Found",
		Detail:     fmt.Sprintf("SSH key with name %q not found", name),
	}
}

// FindExisting searches for an SSH key by name first, then by public key content
func (s *SSHKeys) FindExisting(ctx context.Context, name, publicKey string) (*SSHKey, error) {
	keys, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	// Try by name first
	for _, key := range keys {
		if key.Name == name {
			return &key, nil
		}
	}

	// Fall back to matching by public key content
	for _, key := range keys {
		if key.SSHKey == publicKey {
			return &key, nil
		}
	}

	return nil, &APIError{
		StatusCode: 404,
		Message:    "Not Found",
		Detail:     fmt.Sprintf("SSH key with name %q or matching public key not found", name),
	}
}

// Delete deletes an SSH key by ID
func (s *SSHKeys) Delete(ctx context.Context, id int) error {
	err := s.client.Delete(ctx, fmt.Sprintf("/sshkey/%d", id))
	if err != nil {
		return fmt.Errorf("failed to delete SSH key: %w", err)
	}
	return nil
}
