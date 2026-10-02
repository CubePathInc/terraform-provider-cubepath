package client

import (
	"context"
	"net/url"
)

// ObjectStorageEventDestination is where bucket events are delivered: a signed webhook or a
// Cloud Alerts channel ("notificator"). The webhook URL is never returned in clear.
type ObjectStorageEventDestination struct {
	UUID        string  `json:"uuid"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	URLMasked   *string `json:"url_masked"`
	Notificator *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"notificator"`
	PayloadFormat  string  `json:"payload_format"`
	Status         string  `json:"status"`
	DisabledReason *string `json:"disabled_reason"`
	LastSuccessAt  *string `json:"last_success_at"`
	LastFailureAt  *string `json:"last_failure_at"`
	LastError      *string `json:"last_error"`
	RulesCount     int     `json:"rules_count"`
}

// ObjectStorageEventDestinationSecret is the answer of a create or a secret rotation, the only
// calls that return the signing secret (null for a channel destination).
type ObjectStorageEventDestinationSecret struct {
	Destination   ObjectStorageEventDestination `json:"destination"`
	SigningSecret *string                       `json:"signing_secret"`
}

// CreateObjectStorageEventDestinationRequest creates a webhook (URL) or channel (NotificatorID) destination
type CreateObjectStorageEventDestinationRequest struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	URL           *string `json:"url,omitempty"`
	NotificatorID *string `json:"notificator_id,omitempty"`
	PayloadFormat string  `json:"payload_format,omitempty"`
}

// UpdateObjectStorageEventDestinationRequest changes a destination; nil fields are not sent
type UpdateObjectStorageEventDestinationRequest struct {
	Name          *string `json:"name,omitempty"`
	URL           *string `json:"url,omitempty"`
	PayloadFormat *string `json:"payload_format,omitempty"`
	Enabled       *bool   `json:"enabled,omitempty"`
}

// ObjectStorageEventRule sends a bucket's events to a destination; it is applied asynchronously
type ObjectStorageEventRule struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	BucketUUID  string `json:"bucket_uuid"`
	Destination struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"destination"`
	Events       []string `json:"events"`
	Prefix       string   `json:"prefix"`
	Suffix       string   `json:"suffix"`
	Enabled      bool     `json:"enabled"`
	Status       string   `json:"status"`
	ErrorMessage *string  `json:"error_message"`
}

// CreateObjectStorageEventRuleRequest creates a rule on a bucket
type CreateObjectStorageEventRuleRequest struct {
	Name            string   `json:"name"`
	DestinationUUID string   `json:"destination_uuid"`
	Events          []string `json:"events"`
	Prefix          string   `json:"prefix"`
	Suffix          string   `json:"suffix"`
	Enabled         bool     `json:"enabled"`
}

// UpdateObjectStorageEventRuleRequest changes a rule; nil fields are not sent
type UpdateObjectStorageEventRuleRequest struct {
	Name            *string   `json:"name,omitempty"`
	DestinationUUID *string   `json:"destination_uuid,omitempty"`
	Events          *[]string `json:"events,omitempty"`
	Prefix          *string   `json:"prefix,omitempty"`
	Suffix          *string   `json:"suffix,omitempty"`
	Enabled         *bool     `json:"enabled,omitempty"`
}

const eventDestinationsPath = "/object-storage/event-destinations"

func eventDestinationPath(uuid string) string {
	return eventDestinationsPath + "/" + url.PathEscape(uuid)
}

func eventRulesPath(bucketUUID string) string {
	return "/object-storage/buckets/" + url.PathEscape(bucketUUID) + "/event-rules"
}

// ListEventDestinations retrieves the organization's event destinations
func (s *ObjectStorageService) ListEventDestinations(ctx context.Context) ([]ObjectStorageEventDestination, error) {
	var result []ObjectStorageEventDestination
	if err := s.client.Get(ctx, eventDestinationsPath, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// CreateEventDestination creates an event destination; the answer carries the signing secret
func (s *ObjectStorageService) CreateEventDestination(ctx context.Context, req *CreateObjectStorageEventDestinationRequest) (*ObjectStorageEventDestinationSecret, error) {
	var result ObjectStorageEventDestinationSecret
	if err := s.client.Post(ctx, eventDestinationsPath, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetEventDestination retrieves an event destination
func (s *ObjectStorageService) GetEventDestination(ctx context.Context, uuid string) (*ObjectStorageEventDestination, error) {
	var result ObjectStorageEventDestination
	if err := s.client.Get(ctx, eventDestinationPath(uuid), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateEventDestination changes an event destination
func (s *ObjectStorageService) UpdateEventDestination(ctx context.Context, uuid string, req *UpdateObjectStorageEventDestinationRequest) (*ObjectStorageEventDestination, error) {
	var result ObjectStorageEventDestination
	if err := s.client.Patch(ctx, eventDestinationPath(uuid), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteEventDestination deletes an event destination without rules
func (s *ObjectStorageService) DeleteEventDestination(ctx context.Context, uuid string) error {
	return s.client.Delete(ctx, eventDestinationPath(uuid))
}

// RotateEventDestinationSecret issues a new signing secret; the previous one keeps signing for 24 hours
func (s *ObjectStorageService) RotateEventDestinationSecret(ctx context.Context, uuid string) (*ObjectStorageEventDestinationSecret, error) {
	var result ObjectStorageEventDestinationSecret
	if err := s.client.Post(ctx, eventDestinationPath(uuid)+"/rotate-secret", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TestEventDestination sends a cubepath.ping event to a destination
func (s *ObjectStorageService) TestEventDestination(ctx context.Context, uuid string) error {
	return s.client.Post(ctx, eventDestinationPath(uuid)+"/test", nil, nil)
}

// ListEventRules retrieves the event rules of a bucket
func (s *ObjectStorageService) ListEventRules(ctx context.Context, bucketUUID string) ([]ObjectStorageEventRule, error) {
	var result []ObjectStorageEventRule
	if err := s.client.Get(ctx, eventRulesPath(bucketUUID), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetEventRule finds a rule in the bucket's list (the API has no single rule route); nil when absent
func (s *ObjectStorageService) GetEventRule(ctx context.Context, bucketUUID, ruleUUID string) (*ObjectStorageEventRule, error) {
	rules, err := s.ListEventRules(ctx, bucketUUID)
	if err != nil {
		return nil, err
	}
	for i := range rules {
		if rules[i].UUID == ruleUUID {
			return &rules[i], nil
		}
	}
	return nil, nil
}

// CreateEventRule creates an event rule on a bucket; it starts pending
func (s *ObjectStorageService) CreateEventRule(ctx context.Context, bucketUUID string, req *CreateObjectStorageEventRuleRequest) (*ObjectStorageEventRule, error) {
	var result ObjectStorageEventRule
	if err := s.client.Post(ctx, eventRulesPath(bucketUUID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateEventRule changes an event rule
func (s *ObjectStorageService) UpdateEventRule(ctx context.Context, bucketUUID, ruleUUID string, req *UpdateObjectStorageEventRuleRequest) (*ObjectStorageEventRule, error) {
	var result ObjectStorageEventRule
	if err := s.client.Patch(ctx, eventRulesPath(bucketUUID)+"/"+url.PathEscape(ruleUUID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteEventRule deletes an event rule (asynchronously)
func (s *ObjectStorageService) DeleteEventRule(ctx context.Context, bucketUUID, ruleUUID string) error {
	return s.client.Delete(ctx, eventRulesPath(bucketUUID)+"/"+url.PathEscape(ruleUUID))
}
