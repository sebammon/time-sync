#!/usr/bin/env node

const axios = require("axios");
const { Temporal } = require("temporal-polyfill");
const { Command } = require("commander");
const assert = require("node:assert");
const path = require("path");

require("dotenv").config({ path: path.resolve(__dirname, "../.env") });

// --- CONFIGURATION ---
const {
  CLOCKIFY_API_KEY,
  YOUTRACK_API_KEY,
  CLOCKIFY_WORKSPACE_ID,
  CLOCKIFY_USER_ID,
} = process.env;

const CLOCKIFY_URL = `https://api.clockify.me/api/v1`;
const YOUTRACK_BASE_URL = "https://issues.yournextagency.com/api";

// --- API CALLS ---
const youTrackApi = axios.create({
  baseURL: YOUTRACK_BASE_URL,
  headers: {
    Authorization: `Bearer ${YOUTRACK_API_KEY}`,
  },
});

const clockifyApi = axios.create({
  baseURL: CLOCKIFY_URL,
  headers: {
    "X-Api-Key": CLOCKIFY_API_KEY,
  },
});

async function getBillableEntries(dateRange = {}) {
  const { data } = await clockifyApi.get(
    `/workspaces/${CLOCKIFY_WORKSPACE_ID}/user/${CLOCKIFY_USER_ID}/time-entries`,
    {
      params: { ...dateRange, hydrated: true },
    },
  );

  return (data || []).filter((entry) => entry.billable);
}

async function getWorkspaceProjects() {
  const { data } = await clockifyApi.get(
    `/workspaces/${CLOCKIFY_WORKSPACE_ID}/projects`,
  );
  return data;
}

async function getActiveProjectTasks(projectId) {
  const { data } = await clockifyApi.get(
    `/workspaces/${CLOCKIFY_WORKSPACE_ID}/projects/${projectId}/tasks`,
    { params: { ["is-active"]: true } },
  );

  return data;
}

async function updateProjectTask(projectId, taskId, body) {
  const { data } = await clockifyApi.put(
    `/workspaces/${CLOCKIFY_WORKSPACE_ID}/projects/${projectId}/tasks/${taskId}`,
    body,
  );

  return data;
}

async function getIssueWorkItems(issueId) {
  const items = [];
  let skip = 0;
  const limit = 5000;

  while (true) {
    const { data } = await youTrackApi.get(
      `/issues/${issueId}/timeTracking/workItems?fields=text,date&$skip=${skip}&$top=${limit}`,
    );

    items.push(...data);
    if (data.length < limit) break;
    skip += limit;
  }

  return items;
}

async function getIssue(issueId) {
  const { data } = await youTrackApi.get(`/issues/${issueId}?fields=resolved`);

  return data;
}

async function postIssueWorkItem(
  issueId,
  { durationMinutes, workTypeId, description, date },
) {
  assert.match(description, /\[[a-z\d]{24}]/);

  const payload = {
    duration: { minutes: durationMinutes },
    text: description,
    date: date,
  };

  if (workTypeId) {
    payload.type = { id: workTypeId };
  }

  const { data } = await youTrackApi.post(
    `/issues/${issueId}/timeTracking/workItems`,
    payload,
  );

  return data;
}

async function getAllWorkItemTypes() {
  const { data: workTypes } = await youTrackApi.get(
    `/admin/timeTrackingSettings/workItemTypes?fields=id,name`,
  );

  return workTypes.reduce(
    (acc, type) => ({
      ...acc,
      [type.name.toLowerCase().trim()]: type.id,
    }),
    {},
  );
}

// --- HELPERS ---
function extractIssueId(text) {
  const match = text.trim().match(/^([A-Z0-9]+-\d+)/);

  return match ? match[1] : null;
}

function extractTimeEntryId(text) {
  const match = text.trim().match(/\[([a-z\d]{24})]/);

  return match ? match[1] : null;
}

function createTimeEntry(billableEntry) {
  const { id, description, tags, task, timeInterval } = billableEntry;

  return {
    id: id,
    description: description,
    type: tags?.[0]?.name.toLowerCase().trim() || null,
    issueId: task?.name ? extractIssueId(task.name) : null,
    startDate: timeInterval.start ? new Date(timeInterval.start) : null,
    durationMinutes: timeInterval.duration
      ? Temporal.Duration.from(timeInterval.duration).total("minutes")
      : null,
  };
}

function getUniqueBy(arr, keyFn) {
  return Array.from(new Set(arr.map(keyFn).filter(Boolean)));
}

function getOptionDateRange(now, options) {
  const today = now.startOfDay();

  if (options.today) {
    return {
      start: today,
      end: today.add({ days: 1 }),
    };
  }

  if (options.lastNDays) {
    const start = today.subtract({ days: options.lastNDays });
    return {
      start: start,
      end: today,
    };
  }

  // Yesterday is the default
  return {
    start: today.subtract({ days: 1 }),
    end: today,
  };
}

async function syncEntries(now, options) {
  const { start, end } = getOptionDateRange(now, options);
  const dateRange = {
    start: start.toInstant().toString(),
    end: end.toInstant().toString(),
  };

  console.log(
    `⏳ Syncing entries from ${start.toLocaleString()} to ${end.toLocaleString()}`,
  );

  const entries = await getBillableEntries(dateRange);
  const workTypes = await getAllWorkItemTypes();

  const timeEntries = entries.map(createTimeEntry);

  const uniqueIssueIds = getUniqueBy(
    timeEntries.filter((entry) => entry.issueId),
    (entry) => entry.issueId,
  );

  const workItemsByIssueId = new Map(
    await Promise.all(
      uniqueIssueIds.map(async (issueId) => {
        const issueWorkItems = await getIssueWorkItems(issueId);

        const workItemInternalIds = issueWorkItems
          .map((item) => extractTimeEntryId(item.text || ""))
          .filter(Boolean);

        return [issueId, new Set(workItemInternalIds)];
      }),
    ),
  );

  for (const entry of timeEntries) {
    const displayDate = entry.startDate.toDateString();

    if (!entry.issueId) {
      console.warn(`⚠️ No Issue: ${entry.description} on ${displayDate}`);
      continue;
    }

    const existingItems = workItemsByIssueId.get(entry.issueId);
    const exists = existingItems.has(entry.id);

    if (exists) {
      console.log(
        `⏩ Skipping: ${entry.description} (${entry.issueId}) on ${displayDate}`,
      );
      continue;
    }

    const data = {
      description: `${entry.description} [${entry.id}]`,
      date: entry.startDate.getTime(),
      durationMinutes: entry.durationMinutes,
    };

    if (entry.type && workTypes[entry.type]) {
      data.workTypeId = workTypes[entry.type];
    }

    if (!options.dryRun) {
      await postIssueWorkItem(entry.issueId, data);
    }
    console.log(
      `✅ Synced: ${entry.description} (${entry.issueId}) on ${displayDate}`,
    );
  }
}

// --- MAIN ---
async function main() {
  const program = new Command();

  program
    .option("--today", "Sync only today's entries")
    .option("--yesterday", "Sync only yesterday's entries")
    .option(
      "--last-n-days <n>",
      "Sync entries from the last N days (excluding today)",
      parseInt,
    )
    .option("--dry-run", "Print what would be synced without posting anything")
    .option("--clean-up", "Move all completed tasks into completed")
    .parse();

  const options = program.opts();

  if (options.cleanUp) {
    console.log("⚙️ Running in clean-up mode");

    const workspaceProjects = await getWorkspaceProjects();

    for (const project of workspaceProjects) {
      const projectTasks = await getActiveProjectTasks(project.id);

      for (const task of projectTasks) {
        const issueId = extractIssueId(task.name || "");

        if (!issueId) {
          continue;
        }

        try {
          const ytIssue = await getIssue(issueId);

          if (!ytIssue.resolved) {
            continue;
          }

          await updateProjectTask(project.id, task.id, {
            ...task,
            status: "DONE",
          });
        } catch (e) {
          console.error(e);
          // do nothing for now
        }
      }
    }

    return;
  }

  if (options.dryRun) {
    console.log("⚙️ Running in dry-run mode");
  }

  const now = Temporal.Now.zonedDateTimeISO("UTC");

  await syncEntries(now, options);
}

main().catch((err) => console.error("🛑️ Sync failed:", err));
