package utils

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// StateRefreshFunc is a function that returns the current state of a resource
type StateRefreshFunc func() (result interface{}, state string, err error)

// StateChangeConf is the configuration for waiting on a state change
type StateChangeConf struct {
	Pending      []string          // States that are "in progress"
	Target       []string          // States that are "complete"
	Refresh      StateRefreshFunc  // Function to refresh state
	Timeout      time.Duration     // Maximum time to wait
	PollInterval time.Duration     // Time between polls
	MinTimeout   time.Duration     // Minimum timeout for the first poll
}

// WaitForState waits for the state change to complete
func (conf *StateChangeConf) WaitForState(ctx context.Context) (interface{}, error) {
	if conf.PollInterval == 0 {
		conf.PollInterval = 10 * time.Second
	}

	if conf.MinTimeout == 0 {
		conf.MinTimeout = 5 * time.Second
	}

	// Create a timeout context
	ctx, cancel := context.WithTimeout(ctx, conf.Timeout)
	defer cancel()

	// Create a ticker for polling
	ticker := time.NewTicker(conf.PollInterval)
	defer ticker.Stop()

	// Do first check immediately
	result, state, err := conf.Refresh()
	if err != nil {
		return nil, err
	}

	// Check if we're already in a target state
	if containsString(conf.Target, state) {
		return result, nil
	}

	// Check if we're in an unexpected state
	if !containsString(conf.Pending, state) && !containsString(conf.Target, state) {
		return nil, fmt.Errorf("unexpected state '%s', wanted target '%v'", state, conf.Target)
	}

	// Poll until we reach target state or timeout
	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("timeout waiting for state change")
		case <-ticker.C:
			result, state, err = conf.Refresh()
			if err != nil {
				return nil, err
			}

			// Check if we've reached target state
			if containsString(conf.Target, state) {
				return result, nil
			}

			// Check if we're in an unexpected state
			if !containsString(conf.Pending, state) {
				return nil, fmt.Errorf("unexpected state '%s', wanted target '%v'", state, conf.Target)
			}
		}
	}
}

// containsString checks if a slice contains a string
func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
