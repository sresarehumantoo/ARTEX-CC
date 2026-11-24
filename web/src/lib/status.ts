export type Tone = "neutral" | "blue" | "green" | "amber" | "red" | "rose" | "violet" | "slate";

export const toneClasses: Record<Tone, string> = {
  neutral: "bg-muted text-muted-foreground border-transparent",
  blue: "bg-blue-500/15 text-blue-600 dark:text-blue-400 border-blue-500/20",
  green: "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
  amber: "bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/20",
  red: "bg-red-500/15 text-red-600 dark:text-red-400 border-red-500/20",
  rose: "bg-rose-600 text-white border-rose-600 dark:bg-rose-600 dark:text-white",
  violet: "bg-violet-500/15 text-violet-600 dark:text-violet-400 border-violet-500/20",
  slate: "bg-slate-500/15 text-slate-600 dark:text-slate-400 border-slate-500/20",
};

export const toneDot: Record<Tone, string> = {
  neutral: "bg-muted-foreground",
  blue: "bg-blue-500",
  green: "bg-emerald-500",
  amber: "bg-amber-500",
  red: "bg-red-500",
  rose: "bg-white",
  violet: "bg-violet-500",
  slate: "bg-slate-500",
};

interface StatusMeta {
  label: string;
  tone: Tone;
}

const intent: Record<string, StatusMeta> = {
  open: { label: "Open", tone: "slate" },
  running: { label: "Running", tone: "blue" },
  paused: { label: "Paused", tone: "amber" },
  done: { label: "Done", tone: "green" },
  blocked: { label: "Blocked", tone: "red" },
  exhausted: { label: "Exhausted", tone: "violet" },
  stopped: { label: "Stopped", tone: "slate" },
  deleted: { label: "Deleted", tone: "slate" },
};

const task: Record<string, StatusMeta> = {
  created: { label: "Created", tone: "slate" },
  queued: { label: "Queued", tone: "amber" },
  running: { label: "Running", tone: "blue" },
  paused: { label: "Paused", tone: "amber" },
  done: { label: "Done", tone: "green" },
  failed: { label: "Failed", tone: "red" },
  timeout: { label: "Timed out", tone: "amber" },
};

const severity: Record<string, StatusMeta> = {
  critical: { label: "Critical", tone: "rose" },
  high: { label: "High", tone: "red" },
  medium: { label: "Medium", tone: "amber" },
  low: { label: "Low", tone: "slate" },
  info: { label: "Info", tone: "neutral" },
};

const finding: Record<string, StatusMeta> = {
  pending: { label: "Pending", tone: "amber" },
  in_progress: { label: "In progress", tone: "blue" },
  confirmed: { label: "Confirmed", tone: "red" },
  resolved: { label: "Resolved", tone: "green" },
  fixed: { label: "Fixed", tone: "green" },
  false_positive: { label: "False positive", tone: "slate" },
  ignored: { label: "Ignored", tone: "neutral" },
  duplicate: { label: "Duplicate", tone: "neutral" },
  risk_accepted: { label: "Risk accepted", tone: "violet" },
};

const agent: Record<string, StatusMeta> = {
  exploring: { label: "Exploring", tone: "blue" },
  paused: { label: "Paused", tone: "amber" },
  stalled: { label: "Stalled", tone: "red" },
  idle: { label: "Idle", tone: "neutral" },
};

const goal: Record<string, StatusMeta> = {
  open: { label: "Open", tone: "blue" },
  running: { label: "In progress", tone: "blue" },
  in_progress: { label: "In progress", tone: "blue" },
  met: { label: "Met", tone: "green" },
  disproven: { label: "Disproven", tone: "red" },
  proven: { label: "Proven", tone: "green" },
  abandoned: { label: "Abandoned", tone: "slate" },
};

const node: Record<string, StatusMeta> = {
  observed: { label: "Observed", tone: "slate" },
  confirmed: { label: "Confirmed", tone: "green" },
  tombstoned: { label: "Tombstoned", tone: "neutral" },
};

const approval: Record<string, StatusMeta> = {
  pending: { label: "Pending", tone: "amber" },
  approved: { label: "Approved", tone: "green" },
  allow: { label: "Allowed", tone: "green" },
  denied: { label: "Denied", tone: "red" },
  block: { label: "Blocked", tone: "red" },
};

const activity: Record<string, StatusMeta> = {
  finding: { label: "Finding", tone: "rose" },
  intent: { label: "Intent", tone: "blue" },
  asset: { label: "Asset", tone: "amber" },
  node: { label: "Node", tone: "green" },
  edge: { label: "Edge", tone: "violet" },
  coverage: { label: "Coverage", tone: "blue" },
};

const outcome: Record<string, StatusMeta> = {
  reported: { label: "Reported", tone: "red" },
  no_issue_found: { label: "No issue", tone: "green" },
  ruled_out: { label: "Ruled out", tone: "slate" },
  not_applicable: { label: "N/A", tone: "slate" },
  needs_follow_up: { label: "Follow up", tone: "amber" },
};

const maps = {
  intent,
  task,
  severity,
  finding,
  agent,
  goal,
  node,
  approval,
  activity,
  outcome,
} as const;

export type StatusDomain = keyof typeof maps;

export function statusMeta(domain: StatusDomain, key: string): StatusMeta {
  return maps[domain][key] ?? { label: key, tone: "neutral" };
}
