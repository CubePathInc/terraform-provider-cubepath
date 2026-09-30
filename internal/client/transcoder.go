package client

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// TranscoderS3 is an S3 location of a transcoding job. The secret key is never returned.
type TranscoderS3 struct {
	Endpoint  *string `json:"endpoint"`
	Region    *string `json:"region"`
	Bucket    string  `json:"bucket"`
	Path      string  `json:"path"`
	AccessKey *string `json:"access_key"`
}

// TranscoderJobInput is the input of a transcoding job
type TranscoderJobInput struct {
	Source string        `json:"source"`
	URL    *string       `json:"url"`
	S3     *TranscoderS3 `json:"s3"`
}

// TranscoderJobOutputDestination is where a job writes its outputs
type TranscoderJobOutputDestination struct {
	S3 *TranscoderS3 `json:"s3"`
}

// TranscoderArtifact is a file produced by a finished job
type TranscoderArtifact struct {
	Type   string `json:"type"`
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

// TranscoderJob is a video transcoding job
type TranscoderJob struct {
	UUID              string                          `json:"uuid"`
	Status            string                          `json:"status"`
	Input             *TranscoderJobInput             `json:"input"`
	Output            *TranscoderJobOutputDestination `json:"output"`
	Spec              json.RawMessage                 `json:"spec"`
	Outputs           []TranscoderArtifact            `json:"outputs"`
	Progress          int                             `json:"progress"`
	TotalSegments     int                             `json:"total_segments"`
	CompletedSegments int                             `json:"completed_segments"`
	BatchID           *string                         `json:"batch_id"`
	Error             *string                         `json:"error"`
	CreatedAt         string                          `json:"created_at"`
}

// TranscoderService handles the Video Transcoder API calls
type TranscoderService struct {
	client *Client
}

// NewTranscoderService creates a new Video Transcoder service
func NewTranscoderService(client *Client) *TranscoderService {
	return &TranscoderService{client: client}
}

// ListJobs lists jobs, newest first. batchID filters one bulk submission when not empty.
func (s *TranscoderService) ListJobs(ctx context.Context, batchID string, limit, offset int) ([]TranscoderJob, error) {
	q := url.Values{}
	if batchID != "" {
		q.Set("batch_id", batchID)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	path := "/transcoder/jobs"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var result struct {
		Jobs []TranscoderJob `json:"jobs"`
	}
	if err := s.client.Get(ctx, path, &result); err != nil {
		return nil, err
	}
	return result.Jobs, nil
}

// GetJob retrieves a job by UUID
func (s *TranscoderService) GetJob(ctx context.Context, uuid string) (*TranscoderJob, error) {
	var result TranscoderJob
	if err := s.client.Get(ctx, "/transcoder/jobs/"+url.PathEscape(uuid), &result); err != nil {
		return nil, err
	}
	return &result, nil
}
