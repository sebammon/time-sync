package main

import (
	"fmt"
	"regexp"
	"strings"
)

const youtrackBaseURL = "https://issues.yournextagency.com/api"

type youtrackClient struct {
	http *httpClient
}

func newYouTrackClient(cfg *config) *youtrackClient {
	return &youtrackClient{
		http: newHTTPClient(youtrackBaseURL, map[string]string{
			"Authorization": "Bearer " + cfg.YouTrackAPIKey,
		}),
	}
}

type youtrackWorkItem struct {
	Text string `json:"text"`
	Date int64  `json:"date"`
}

type youtrackIssue struct {
	Resolved *int64 `json:"resolved"`
}

type youtrackWorkItemType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (y *youtrackClient) getIssueWorkItems(issueID string) ([]youtrackWorkItem, error) {
	const limit = 5000
	skip := 0
	var all []youtrackWorkItem

	for {
		var page []youtrackWorkItem
		path := fmt.Sprintf("/issues/%s/timeTracking/workItems?fields=text,date&$skip=%d&$top=%d", issueID, skip, limit)
		if err := y.http.get(path, &page); err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < limit {
			break
		}
		skip += limit
	}
	return all, nil
}

func (y *youtrackClient) getIssue(issueID string) (*youtrackIssue, error) {
	var issue youtrackIssue
	err := y.http.get(fmt.Sprintf("/issues/%s?fields=resolved", issueID), &issue)
	if err != nil {
		return nil, err
	}
	return &issue, nil
}

var entryIDInDescriptionRe = regexp.MustCompile(`\[[a-z\d]{24}\]`)

type postWorkItemArgs struct {
	durationMinutes int
	workTypeID      string
	description     string
	dateMS          int64
}

func (y *youtrackClient) postIssueWorkItem(issueID string, args postWorkItemArgs) error {
	if !entryIDInDescriptionRe.MatchString(args.description) {
		return fmt.Errorf("description missing [entryID] marker: %q", args.description)
	}

	payload := map[string]any{
		"duration": map[string]int{"minutes": args.durationMinutes},
		"text":     args.description,
		"date":     args.dateMS,
	}
	if args.workTypeID != "" {
		payload["type"] = map[string]string{"id": args.workTypeID}
	}

	return y.http.post(fmt.Sprintf("/issues/%s/timeTracking/workItems", issueID), payload, nil)
}

func (y *youtrackClient) getAllWorkItemTypes() (map[string]string, error) {
	var types []youtrackWorkItemType
	if err := y.http.get("/admin/timeTrackingSettings/workItemTypes?fields=id,name", &types); err != nil {
		return nil, err
	}

	out := make(map[string]string, len(types))
	for _, t := range types {
		out[strings.ToLower(strings.TrimSpace(t.Name))] = t.ID
	}
	return out, nil
}
