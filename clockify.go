package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const clockifyBaseURL = "https://api.clockify.me/api/v1"

type clockifyClient struct {
	http        *httpClient
	workspaceID string
	userID      string
}

func newClockifyClient(cfg *config) *clockifyClient {
	return &clockifyClient{
		http: newHTTPClient(clockifyBaseURL, map[string]string{
			"X-Api-Key": cfg.ClockifyAPIKey,
		}),
		workspaceID: cfg.ClockifyWorkspaceID,
		userID:      cfg.ClockifyUserID,
	}
}

type clockifyTimeInterval struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Duration string `json:"duration"`
}

type clockifyTag struct {
	Name string `json:"name"`
}

type clockifyTask struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type clockifyEntry struct {
	ID           string               `json:"id"`
	Description  string               `json:"description"`
	Billable     bool                 `json:"billable"`
	Tags         []clockifyTag        `json:"tags"`
	Task         *clockifyTask        `json:"task"`
	TimeInterval clockifyTimeInterval `json:"timeInterval"`
}

type clockifyProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *clockifyClient) getBillableEntries(start, end string) ([]clockifyEntry, error) {
	q := url.Values{}
	if start != "" {
		q.Set("start", start)
	}
	if end != "" {
		q.Set("end", end)
	}
	q.Set("hydrated", "true")

	path := fmt.Sprintf("/workspaces/%s/user/%s/time-entries?%s", c.workspaceID, c.userID, q.Encode())

	var entries []clockifyEntry
	if err := c.http.get(path, &entries); err != nil {
		return nil, err
	}

	out := entries[:0]
	for _, e := range entries {
		if e.Billable {
			out = append(out, e)
		}
	}
	return out, nil
}

func (c *clockifyClient) getWorkspaceProjects() ([]clockifyProject, error) {
	var projects []clockifyProject
	err := c.http.get(fmt.Sprintf("/workspaces/%s/projects", c.workspaceID), &projects)
	return projects, err
}

func (c *clockifyClient) getActiveProjectTasks(projectID string) ([]map[string]any, error) {
	var tasks []map[string]any
	path := fmt.Sprintf("/workspaces/%s/projects/%s/tasks?is-active=true", c.workspaceID, projectID)
	err := c.http.get(path, &tasks)
	return tasks, err
}

func (c *clockifyClient) updateProjectTask(projectID, taskID string, body map[string]any) error {
	path := fmt.Sprintf("/workspaces/%s/projects/%s/tasks/%s", c.workspaceID, projectID, taskID)
	return c.http.put(path, body, nil)
}

var isoDurationRe = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)

// parseISO8601DurationMinutes parses durations like PT1H30M, PT45M, PT2H, PT0S → total minutes.
// Seconds are truncated (matches Temporal.Duration.total("minutes") behavior closely for billing).
func parseISO8601DurationMinutes(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	m := isoDurationRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("unrecognized ISO 8601 duration %q", s)
	}
	parse := func(s string) int {
		if s == "" {
			return 0
		}
		n, _ := strconv.Atoi(s)
		return n
	}
	hours := parse(m[1])
	mins := parse(m[2])
	// seconds truncated
	return hours*60 + mins, nil
}
