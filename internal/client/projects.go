package client

import (
	"context"
	"fmt"
)

// Projects provides methods for managing projects
type Projects struct {
	client *Client
}

// NewProjects creates a new projects service
func NewProjects(client *Client) *Projects {
	return &Projects{client: client}
}

// Create creates a new project
func (p *Projects) Create(ctx context.Context, req *CreateProjectRequest) (*Project, error) {
	// API only returns {"detail": "Project 'name' created successfully"}
	// So we need to fetch the created project by name
	var createResp struct {
		Detail string `json:"detail"`
	}
	err := p.client.Post(ctx, "/projects/", req, &createResp)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	// Fetch all projects and find the one we just created
	projects, err := p.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("project created but failed to fetch details: %w", err)
	}

	// Find project by name (most recently created with matching name)
	for i := len(projects) - 1; i >= 0; i-- {
		if projects[i].Project.Name == req.Name {
			return &projects[i].Project, nil
		}
	}

	return nil, fmt.Errorf("project created but not found in list")
}

// List retrieves all projects with their resources
func (p *Projects) List(ctx context.Context) ([]ProjectResponse, error) {
	var result []ProjectResponse
	err := p.client.Get(ctx, "/projects/", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	return result, nil
}

// Get retrieves a specific project by ID
func (p *Projects) Get(ctx context.Context, id int) (*ProjectResponse, error) {
	projects, err := p.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, projectResp := range projects {
		if projectResp.Project.ID == id {
			return &projectResp, nil
		}
	}

	return nil, &APIError{
		StatusCode: 404,
		Message:    "Not Found",
		Detail:     fmt.Sprintf("project with ID %d not found", id),
	}
}

// Update updates a project (description only)
func (p *Projects) Update(ctx context.Context, id int, req *CreateProjectRequest) (*Project, error) {
	var result Project
	err := p.client.Put(ctx, fmt.Sprintf("/projects/%d", id), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}
	return &result, nil
}

// Delete deletes a project by ID
func (p *Projects) Delete(ctx context.Context, id int) error {
	err := p.client.Delete(ctx, fmt.Sprintf("/projects/%d", id))
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	return nil
}
