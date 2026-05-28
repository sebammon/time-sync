package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

type timeEntry struct {
	id              string
	description     string
	entryType       string
	issueID         string
	startDate       time.Time
	durationMinutes int
}

var (
	issueIDRe     = regexp.MustCompile(`^([A-Z0-9]+-\d+)`)
	timeEntryIDRe = regexp.MustCompile(`\[([a-z\d]{24})\]`)
)

func extractIssueID(text string) string {
	m := issueIDRe.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return ""
	}
	return m[1]
}

func extractTimeEntryID(text string) string {
	m := timeEntryIDRe.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return ""
	}
	return m[1]
}

func toTimeEntry(e clockifyEntry) timeEntry {
	te := timeEntry{
		id:          e.ID,
		description: e.Description,
	}
	if len(e.Tags) > 0 {
		te.entryType = strings.ToLower(strings.TrimSpace(e.Tags[0].Name))
	}
	if e.Task != nil {
		te.issueID = extractIssueID(e.Task.Name)
	}
	if e.TimeInterval.Start != "" {
		if t, err := time.Parse(time.RFC3339, e.TimeInterval.Start); err == nil {
			te.startDate = t
		}
	}
	if e.TimeInterval.Duration != "" {
		if m, err := parseISO8601DurationMinutes(e.TimeInterval.Duration); err == nil {
			te.durationMinutes = m
		}
	}
	return te
}

type dateRange struct {
	start time.Time
	end   time.Time
}

func getOptionDateRange(now time.Time, opts syncOptions) dateRange {
	today := now.Truncate(24 * time.Hour)

	switch {
	case opts.today:
		return dateRange{start: today, end: today.AddDate(0, 0, 1)}
	case opts.lastNDays > 0:
		return dateRange{start: today.AddDate(0, 0, -opts.lastNDays), end: today}
	default:
		return dateRange{start: today.AddDate(0, 0, -1), end: today}
	}
}

func syncEntries(clockify *clockifyClient, youtrack *youtrackClient, now time.Time, opts syncOptions) error {
	rng := getOptionDateRange(now, opts)

	fmt.Printf("⏳ Syncing entries from %s to %s\n",
		rng.start.Local().Format("Mon Jan 02 2006 15:04:05 MST"),
		rng.end.Local().Format("Mon Jan 02 2006 15:04:05 MST"),
	)

	entries, err := clockify.getBillableEntries(
		rng.start.UTC().Format(time.RFC3339),
		rng.end.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("fetch billable entries: %w", err)
	}

	workTypes, err := youtrack.getAllWorkItemTypes()
	if err != nil {
		return fmt.Errorf("fetch work item types: %w", err)
	}

	timeEntries := make([]timeEntry, 0, len(entries))
	for _, e := range entries {
		timeEntries = append(timeEntries, toTimeEntry(e))
	}

	seenIssues := make(map[string]bool)
	for _, te := range timeEntries {
		if te.issueID != "" {
			seenIssues[te.issueID] = true
		}
	}

	workItemsByIssue := make(map[string]map[string]bool, len(seenIssues))
	for issueID := range seenIssues {
		items, err := youtrack.getIssueWorkItems(issueID)
		if err != nil {
			return fmt.Errorf("fetch work items for %s: %w", issueID, err)
		}
		idSet := make(map[string]bool)
		for _, item := range items {
			if id := extractTimeEntryID(item.Text); id != "" {
				idSet[id] = true
			}
		}
		workItemsByIssue[issueID] = idSet
	}

	for _, entry := range timeEntries {
		displayDate := entry.startDate.Local().Format("Mon Jan 02 2006")

		if entry.issueID == "" {
			fmt.Printf("⚠️ No Issue: %s on %s\n", entry.description, displayDate)
			continue
		}

		existing := workItemsByIssue[entry.issueID]
		if existing[entry.id] {
			fmt.Printf("⏩ Skipping: %s (%s) on %s\n", entry.description, entry.issueID, displayDate)
			continue
		}

		args := postWorkItemArgs{
			description:     fmt.Sprintf("%s [%s]", entry.description, entry.id),
			dateMS:          entry.startDate.UnixMilli(),
			durationMinutes: entry.durationMinutes,
		}
		if entry.entryType != "" {
			if id, ok := workTypes[entry.entryType]; ok {
				args.workTypeID = id
			}
		}

		if !opts.dryRun {
			if err := youtrack.postIssueWorkItem(entry.issueID, args); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Failed to sync %s (%s): %v\n", entry.description, entry.issueID, err)
				continue
			}
		}
		fmt.Printf("✅ Synced: %s (%s) on %s\n", entry.description, entry.issueID, displayDate)
	}

	return nil
}

func cleanUp(clockify *clockifyClient, youtrack *youtrackClient) error {
	projects, err := clockify.getWorkspaceProjects()
	if err != nil {
		return fmt.Errorf("fetch projects: %w", err)
	}

	for _, project := range projects {
		tasks, err := clockify.getActiveProjectTasks(project.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error fetching tasks for %s: %v\n", project.ID, err)
			continue
		}

		for _, task := range tasks {
			name, _ := task["name"].(string)
			issueID := extractIssueID(name)
			if issueID == "" {
				continue
			}

			issue, err := youtrack.getIssue(issueID)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			if issue.Resolved == nil {
				continue
			}

			body := make(map[string]any, len(task)+1)
			for k, v := range task {
				body[k] = v
			}
			body["status"] = "DONE"

			taskID, _ := task["id"].(string)
			if err := clockify.updateProjectTask(project.ID, taskID, body); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}

	return nil
}
