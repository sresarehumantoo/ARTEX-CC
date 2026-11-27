package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/store"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

// The patterns below match the result text of tools/intent_tools.go and
// tools/claim.go (ClaimConflictError). A holder label is either a hashed
// anonymous label (h-...) or an identity name, optionally followed by
// /h-... when an owner subdivides it.
var (
	claimRe      = regexp.MustCompile(`intent (\d+) is now running; (claimed|renewed) by ([a-z0-9][a-z0-9._/-]*) until (\S+)`)
	conflictRe   = regexp.MustCompile(`intent (\d+) is claimed by ([a-z0-9][a-z0-9._/-]*) until (\S+)`)
	evalHolderRe = regexp.MustCompile(`^eval/[^/]+/i(\d+)(/|$)`)
	doneRe       = regexp.MustCompile(`intent (\d+) (marked done|cancelled)`)
	createdRe    = regexp.MustCompile(`^(intent|fact) created: (\d+)$`)
)

const conflictWindow = 5

// renewalConflictText is in every ClaimConflictError from servers that renew
// claims implicitly (record_fact, report_finding, record_coverage), and in no
// earlier server's output.
const renewalConflictText = "The claim renews while its holder records results against it"

// ToolCall is one tool_use from a stream-json transcript with its result.
type ToolCall struct {
	Tool    string
	Input   json.RawMessage
	Result  string
	IsError bool
	At      time.Time
}

type Usage struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadTokens     int64 `json:"cache_read_input_tokens"`
}

func (u *Usage) add(o Usage) {
	u.InputTokens += o.InputTokens
	u.OutputTokens += o.OutputTokens
	u.CacheCreationTokens += o.CacheCreationTokens
	u.CacheReadTokens += o.CacheReadTokens
}

// Transcript is one instance's parsed claude --output-format stream-json output.
type Transcript struct {
	Instance  int
	Calls     []ToolCall
	Usage     Usage
	CostUSD   float64
	HasResult bool
}

type streamLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		ID      string          `json:"id"`
		Content json.RawMessage `json:"content"`
		Usage   *Usage          `json:"usage"`
	} `json:"message"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        *Usage  `json:"usage"`
}

type contentBlock struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

func parseTranscript(instance int, path string) (*Transcript, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	t := &Transcript{Instance: instance}
	byID := map[string]int{}
	seenMsg := map[string]bool{}
	var msgUsage Usage
	var resultUsage *Usage
	var last time.Time

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 256<<20)
	for sc.Scan() {
		var line streamLine
		if json.Unmarshal(sc.Bytes(), &line) != nil {
			continue
		}
		if ts, err := time.Parse(time.RFC3339Nano, line.Timestamp); err == nil {
			last = ts
		}
		switch line.Type {
		case "assistant":
			// One API message can arrive as several lines that repeat its usage.
			if line.Message.Usage != nil && !seenMsg[line.Message.ID] {
				seenMsg[line.Message.ID] = true
				msgUsage.add(*line.Message.Usage)
			}
			var blocks []contentBlock
			_ = json.Unmarshal(line.Message.Content, &blocks)
			for _, b := range blocks {
				if b.Type != "tool_use" {
					continue
				}
				byID[b.ID] = len(t.Calls)
				t.Calls = append(t.Calls, ToolCall{Tool: shortToolName(b.Name), Input: b.Input, At: last})
			}
		case "user":
			var blocks []contentBlock
			_ = json.Unmarshal(line.Message.Content, &blocks)
			for _, b := range blocks {
				i, ok := byID[b.ToolUseID]
				if b.Type != "tool_result" || !ok {
					continue
				}
				t.Calls[i].Result = resultText(b.Content)
				t.Calls[i].IsError = b.IsError
				t.Calls[i].At = last
			}
		case "result":
			t.HasResult = true
			// Each later turn (a background task waking the session) writes another
			// result line; total_cost_usd is the session's running total, while usage
			// covers that turn alone.
			t.CostUSD = line.TotalCostUSD
			if line.Usage != nil {
				if resultUsage == nil {
					resultUsage = &Usage{}
				}
				resultUsage.add(*line.Usage)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	t.Usage = msgUsage
	if resultUsage != nil {
		t.Usage = *resultUsage
	}
	return t, nil
}

func shortToolName(name string) string {
	if strings.HasPrefix(name, "mcp__") {
		if i := strings.LastIndex(name, "__"); i > len("mcp_") {
			return name[i+2:]
		}
	}
	return name
}

func resultText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	_ = json.Unmarshal(raw, &parts)
	var texts []string
	for _, p := range parts {
		if p.Type == "text" {
			texts = append(texts, p.Text)
		}
	}
	return strings.Join(texts, "\n")
}

func inputNodeID(raw json.RawMessage) int64 {
	var a struct {
		NodeID json.RawMessage `json:"node_id"`
	}
	_ = json.Unmarshal(raw, &a)
	n, _ := strconv.ParseInt(strings.Trim(string(a.NodeID), `"`), 10, 64)
	return n
}

func inputIntentID(raw json.RawMessage) int64 {
	var a struct {
		IntentID json.RawMessage `json:"intent_id"`
	}
	_ = json.Unmarshal(raw, &a)
	n, _ := strconv.ParseInt(strings.Trim(string(a.IntentID), `"`), 10, 64)
	return n
}

// EngagementFinding is a finding in the final engagement with its assets.
type EngagementFinding struct {
	ID         int64
	VulnClass  string
	AssetIDs   []int64
	MergedInto int64
	Dismissed  bool
}

type CoordinationInput struct {
	Transcripts []*Transcript
	Findings    []EngagementFinding
	Edges       []tools.Edge
	Kills       map[int]time.Time
	Lease       time.Duration
	// Renewal says the server renews claims implicitly, as probed from its
	// tool catalog. Transcripts showing it turn renewal on as well.
	Renewal bool
}

type Coordination struct {
	DoubleClaims       []DoubleClaim       `json:"double_claims"`
	Conflicts          int                 `json:"conflicts"`
	ConflictsHandled   int                 `json:"conflicts_handled"`
	ConflictsHandledPc float64             `json:"conflicts_handled_pct"`
	UnhandledConflicts []ConflictRef       `json:"unhandled_conflicts,omitempty"`
	RefusedCompletions int                 `json:"refused_completions"`
	HolderMismatches   int                 `json:"holder_mismatches"`
	DuplicateFindings  DuplicateFindings   `json:"duplicate_findings"`
	RedundantRecon     RedundantRecon      `json:"redundant_recon"`
	CrossParents       CrossParents        `json:"cross_instance_parents"`
	Takeovers          []Takeover          `json:"takeovers,omitempty"`
	UnclaimedWork      int                 `json:"unclaimed_work_calls"`
	UnclaimedFinishes  int                 `json:"unclaimed_finishes"`
	OwnCancels         int                 `json:"own_duplicate_cancels"`
	ConcurrentWork     []ConcurrentWork    `json:"concurrent_intent_work"`
	ImplicitRenewal    bool                `json:"implicit_renewal"`
	ImplicitRenewals   int                 `json:"implicit_renewals"`
	Instances          []InstanceCoord     `json:"instances"`
	Total              InstanceCoordTotals `json:"total"`
}

type DoubleClaim struct {
	IntentID       int64     `json:"intent_id"`
	FirstInstance  int       `json:"first_instance"`
	FirstHolder    string    `json:"first_holder"`
	FirstExpires   time.Time `json:"first_expires"`
	SecondInstance int       `json:"second_instance"`
	SecondHolder   string    `json:"second_holder"`
	SecondAt       time.Time `json:"second_at"`
}

type ConflictRef struct {
	Instance       int    `json:"instance"`
	Call           int    `json:"call"`
	IntentID       int64  `json:"intent_id"`
	Holder         string `json:"holder,omitempty"`
	HolderInstance int    `json:"holder_instance,omitempty"`
}

type DuplicateFindings struct {
	Raw         int              `json:"raw"`
	AfterMerges int              `json:"after_merges"`
	Groups      []DuplicateGroup `json:"groups,omitempty"`
}

type DuplicateGroup struct {
	VulnClass  string  `json:"vulnclass"`
	AssetID    int64   `json:"asset_id"`
	FindingIDs []int64 `json:"finding_ids"`
	Instances  []int   `json:"instances"`
	Live       int     `json:"live"`
}

type RedundantRecon struct {
	SharedCommands int             `json:"shared_commands"`
	RedundantRuns  int             `json:"redundant_runs"`
	Examples       []SharedCommand `json:"examples,omitempty"`
}

type SharedCommand struct {
	Command   string `json:"command"`
	Instances []int  `json:"instances"`
	Runs      int    `json:"runs"`
}

type CrossParents struct {
	Total  int            `json:"total"`
	ByKind map[string]int `json:"by_child_kind"`
}

type Takeover struct {
	IntentID         int64      `json:"intent_id"`
	KilledInstance   int        `json:"killed_instance"`
	LeaseExpires     time.Time  `json:"lease_expires"`
	TakenBy          int        `json:"taken_by,omitempty"`
	TakenAt          *time.Time `json:"taken_at,omitempty"`
	AfterLeaseLapsed bool       `json:"after_lease_lapsed"`
	FactsRecorded    int        `json:"facts_recorded"`
	FactsAttached    int        `json:"facts_attached"`
}

// ConcurrentWork is two instances whose work windows on one intent overlap
// between From and To (see concurrentWork).
type ConcurrentWork struct {
	IntentID       int64     `json:"intent_id"`
	FirstInstance  int       `json:"first_instance"`
	FirstRecords   int       `json:"first_records"`
	SecondInstance int       `json:"second_instance"`
	SecondRecords  int       `json:"second_records"`
	From           time.Time `json:"overlap_from"`
	To             time.Time `json:"overlap_to"`
}

type UnclaimedWork struct {
	WorkCalls  int     `json:"work_calls"`
	Calls      int     `json:"calls"`
	Bash       int     `json:"bash"`
	Records    int     `json:"artex_records"`
	Finishes   []int64 `json:"finished_unclaimed_intents,omitempty"`
	OwnCancels []int64 `json:"own_duplicate_cancels,omitempty"`
}

type InstanceCoord struct {
	Instance      int           `json:"instance"`
	ToolCalls     int           `json:"tool_calls"`
	BashCommands  int           `json:"bash_commands"`
	ListChanges   int           `json:"list_changes_calls"`
	ClaimsTaken   int           `json:"claims_taken"`
	Conflicts     int           `json:"conflicts"`
	Holders       []string      `json:"holders,omitempty"`
	NodesCreated  int           `json:"nodes_created"`
	CrossParents  int           `json:"cross_instance_parents"`
	Unclaimed     UnclaimedWork `json:"unclaimed_work"`
	Usage         Usage         `json:"usage"`
	CostUSD       float64       `json:"cost_usd"`
	ResultMessage bool          `json:"result_message"`
}

type InstanceCoordTotals struct {
	Usage   Usage   `json:"usage"`
	CostUSD float64 `json:"cost_usd"`
}

type claimEvent struct {
	instance int
	call     int
	intent   int64
	holder   string
	at       time.Time
	expires  time.Time
	renewed  bool
	implicit bool
}

type opKind int

const (
	opClaim opKind = iota
	opRelease
	opRenew
)

// claimOp is one claim, release or renewing call, in transcript order.
type claimOp struct {
	kind opKind
	claimEvent
}

type pendingConflict struct {
	ti, call int
	intent   int64
	until    time.Time
	holder   string
}

type releaseEvent struct {
	intent   int64
	at       time.Time
	instance int
}

type nodeOrigin struct {
	instance int
	kind     string
	intent   int64
}

func scoreCoordination(in CoordinationInput) *Coordination {
	c := &Coordination{CrossParents: CrossParents{ByKind: map[string]int{}}}
	lease := in.Lease
	if lease <= 0 {
		lease = 30 * time.Minute
	}

	var claims []claimEvent
	var releases []releaseEvent
	var ops []claimOp
	var conflicts []pendingConflict
	renewal := in.Renewal
	nodes := map[int64]nodeOrigin{}
	findingOwner := map[int64]int{}
	commands := map[string]map[int]int{}
	holderInstance := map[string]int{}
	perInstance := make([]InstanceCoord, len(in.Transcripts))
	var firstIntent time.Time

	for ti, t := range in.Transcripts {
		ic := &perInstance[ti]
		ic.Instance = t.Instance
		ic.ToolCalls = len(t.Calls)
		ic.Usage = t.Usage
		ic.CostUSD = t.CostUSD
		ic.ResultMessage = t.HasResult
		c.Total.Usage.add(t.Usage)
		c.Total.CostUSD += t.CostUSD

		for i, call := range t.Calls {
			if strings.Contains(call.Result, renewalConflictText) {
				renewal = true
			}
			renew := func(intent int64) {
				ops = append(ops, claimOp{opRenew, claimEvent{instance: t.Instance, call: i, intent: intent, at: call.At}})
			}
			switch call.Tool {
			case "start_intent":
				if m := claimRe.FindStringSubmatch(call.Result); m != nil && !call.IsError {
					id, _ := strconv.ParseInt(m[1], 10, 64)
					exp, err := time.Parse(time.RFC3339, m[4])
					if err != nil {
						exp = call.At.Add(lease)
					}
					cl := claimEvent{instance: t.Instance, call: i, intent: id, holder: m[3], at: call.At, expires: exp, renewed: m[2] == "renewed"}
					claims = append(claims, cl)
					ops = append(ops, claimOp{opClaim, cl})
					ic.ClaimsTaken++
					if _, seen := holderInstance[m[3]]; !seen {
						holderInstance[m[3]] = t.Instance
					}
					if !slices.Contains(ic.Holders, m[3]) {
						ic.Holders = append(ic.Holders, m[3])
					}
					if n := evalHolderInstance(m[3]); n > 0 && n != t.Instance {
						c.HolderMismatches++
					}
				} else if m := conflictRe.FindStringSubmatch(call.Result); m != nil {
					id, _ := strconv.ParseInt(m[1], 10, 64)
					until, _ := time.Parse(time.RFC3339, m[3])
					c.Conflicts++
					ic.Conflicts++
					conflicts = append(conflicts, pendingConflict{ti, i, id, until, m[2]})
				}
			case "complete_intent", "cancel_intent":
				if conflictRe.MatchString(call.Result) {
					c.RefusedCompletions++
				} else if m := doneRe.FindStringSubmatch(call.Result); m != nil && !call.IsError {
					id, _ := strconv.ParseInt(m[1], 10, 64)
					releases = append(releases, releaseEvent{id, call.At, t.Instance})
					ops = append(ops, claimOp{opRelease, claimEvent{instance: t.Instance, call: i, intent: id, at: call.At}})
				}
			case "list_changes":
				ic.ListChanges++
			case "Bash":
				var a struct {
					Command string `json:"command"`
				}
				if json.Unmarshal(call.Input, &a) == nil && a.Command != "" {
					ic.BashCommands++
					cmd := strings.Join(strings.Fields(a.Command), " ")
					if commands[cmd] == nil {
						commands[cmd] = map[int]int{}
					}
					commands[cmd][t.Instance]++
				}
			case "add_intent", "record_fact", "report_finding":
				if call.IsError {
					continue
				}
				for _, n := range createdNodes(call) {
					if n.kind == tools.KindIntent && (firstIntent.IsZero() || call.At.Before(firstIntent)) {
						firstIntent = call.At
					}
					nodes[n.id] = nodeOrigin{t.Instance, n.kind, n.intent}
					ic.NodesCreated++
					if n.findingID > 0 {
						findingOwner[n.findingID] = t.Instance
					}
					if n.intent > 0 {
						renew(n.intent)
					}
				}
			case "record_coverage":
				if id := inputIntentID(call.Input); id > 0 && !call.IsError {
					renew(id)
				}
			}
		}
	}

	if renewal {
		c.ImplicitRenewal = true
		n := len(claims)
		claims = replayRenewals(claims, ops, lease)
		c.ImplicitRenewals = len(claims) - n
	}
	for _, pc := range conflicts {
		t := in.Transcripts[pc.ti]
		var heldByOther func(time.Time) bool
		if renewal {
			heldByOther = func(at time.Time) bool { return heldByOtherInstance(claims, releases, pc.intent, t.Instance, at) }
		}
		if conflictHandled(t.Calls, pc.call, pc.intent, pc.until, heldByOther) {
			c.ConflictsHandled++
		} else {
			c.UnhandledConflicts = append(c.UnhandledConflicts, ConflictRef{Instance: t.Instance, Call: pc.call, IntentID: pc.intent, Holder: pc.holder})
		}
	}
	for k, u := range c.UnhandledConflicts {
		c.UnhandledConflicts[k].HolderInstance = resolveHolder(u.Holder, holderInstance)
	}
	c.DoubleClaims = doubleClaims(claims, releases)
	c.ConcurrentWork = concurrentWork(in.Transcripts, claims, releases)
	for ti, t := range in.Transcripts {
		u := unclaimedWork(t, claims, releases, nodes, firstIntent)
		perInstance[ti].Unclaimed = u
		c.UnclaimedWork += u.Calls
		c.UnclaimedFinishes += len(u.Finishes)
		c.OwnCancels += len(u.OwnCancels)
	}
	if c.Conflicts > 0 {
		c.ConflictsHandledPc = 100 * float64(c.ConflictsHandled) / float64(c.Conflicts)
	}
	c.DuplicateFindings = duplicateFindings(in.Findings, findingOwner)
	c.RedundantRecon = redundantRecon(commands)

	crossByInstance := map[int]int{}
	for _, e := range in.Edges {
		child, ok1 := nodes[e.To]
		parent, ok2 := nodes[e.From]
		if ok1 && ok2 && child.instance != parent.instance {
			c.CrossParents.Total++
			c.CrossParents.ByKind[child.kind]++
			crossByInstance[child.instance]++
		}
	}
	for i := range perInstance {
		perInstance[i].CrossParents = crossByInstance[perInstance[i].Instance]
	}
	c.Instances = perInstance

	for inst, at := range in.Kills {
		c.Takeovers = append(c.Takeovers, takeovers(inst, at, lastCallTime(in.Transcripts, inst), claims, releases, nodes, in.Edges, lease)...)
	}
	return c
}

// evalHolderInstance returns the instance number in an eval runner identity
// label (eval/<run>/i<n>, optionally /h-...), or 0 for any other label.
func evalHolderInstance(label string) int {
	m := evalHolderRe.FindStringSubmatch(label)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// resolveHolder names the instance behind a holder label: from the label
// itself for eval identities, else from the instance whose own successful
// claims carried that label. Anonymous labels never claimed in a transcript
// resolve to 0.
func resolveHolder(label string, claimedBy map[string]int) int {
	if n := evalHolderInstance(label); n > 0 {
		return n
	}
	return claimedBy[label]
}

// conflictHandled reports whether an agent left a claimed intent alone after
// being told another holder has it. Retrying the same intent, or completing,
// canceling or recording work against it within the next conflictWindow
// calls, is mishandling. Moving to another intent, carrying on with a claim it
// already holds, or polling and waiting when nothing is left are all fine, as
// is claiming the intent again once the other holder's lease (until) lapsed.
// With heldByOther, the lease must also have lapsed in the replay, since the
// holder may have renewed it after the conflict.
func conflictHandled(calls []ToolCall, i int, intent int64, until time.Time, heldByOther func(time.Time) bool) bool {
	for j := i + 1; j < len(calls) && j <= i+conflictWindow; j++ {
		switch calls[j].Tool {
		case "start_intent":
			if inputNodeID(calls[j].Input) == intent {
				at := calls[j].At
				return !until.IsZero() && !at.Before(until) && (heldByOther == nil || !heldByOther(at))
			}
		case "complete_intent", "cancel_intent":
			if inputNodeID(calls[j].Input) == intent {
				return false
			}
		case "report_finding", "record_fact", "record_coverage":
			if inputIntentID(calls[j].Input) == intent {
				return false
			}
		}
	}
	return true
}

// lastCallTime is the time of the instance's last recorded tool call, the
// closest proxy for when it was last working before a kill.
func lastCallTime(ts []*Transcript, instance int) time.Time {
	var last time.Time
	for _, t := range ts {
		if t.Instance != instance {
			continue
		}
		for _, c := range t.Calls {
			if c.At.After(last) {
				last = c.At
			}
		}
	}
	return last
}

// claimedByOtherBetween reports whether an instance other than the given one
// claimed the intent after from (exclusive) and at or before to.
func claimedByOtherBetween(claims []claimEvent, instance int, intent int64, from, to time.Time) bool {
	for _, cl := range claims {
		if cl.intent == intent && cl.instance != instance && cl.at.After(from) && !cl.at.After(to) {
			return true
		}
	}
	return false
}

func releasedBetween(releases []releaseEvent, intent int64, from, to time.Time) bool {
	for _, r := range releases {
		if r.intent == intent && !r.at.Before(from) && r.at.Before(to) {
			return true
		}
	}
	return false
}

// replayRenewals adds the implicit renewals the server's RenewIntentClaims
// makes for record_fact, report_finding and record_coverage calls: only when
// the caller's instance made the intent's latest claim, nothing released it
// since, and that claim had less than half a lease left. A lapsed claim nobody
// else took is revived. Each renewal is a claim by the same holder from the
// call to call time + lease.
func replayRenewals(claims []claimEvent, ops []claimOp, lease time.Duration) []claimEvent {
	sorted := append([]claimOp(nil), ops...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].at.Before(sorted[j].at) })
	held := map[int64]claimEvent{}
	out := append([]claimEvent(nil), claims...)
	for _, op := range sorted {
		switch op.kind {
		case opClaim:
			held[op.intent] = op.claimEvent
		case opRelease:
			delete(held, op.intent)
		case opRenew:
			h, ok := held[op.intent]
			if !ok || h.instance != op.instance || !h.expires.Before(op.at.Add(lease/2)) {
				continue
			}
			h = claimEvent{instance: op.instance, call: op.call, intent: op.intent, holder: h.holder,
				at: op.at, expires: op.at.Add(lease), renewed: true, implicit: true}
			held[op.intent] = h
			out = append(out, h)
		}
	}
	return out
}

func heldByOtherInstance(claims []claimEvent, releases []releaseEvent, intent int64, instance int, at time.Time) bool {
	for _, cl := range claims {
		if cl.intent == intent && cl.instance != instance && !cl.at.After(at) && at.Before(cl.expires) &&
			!releasedBetween(releases, intent, cl.at, at) {
			return true
		}
	}
	return false
}

// doubleClaims replays successful claims in time order and flags a claim by a
// second instance while the previous holder's lease was still live.
func doubleClaims(claims []claimEvent, releases []releaseEvent) []DoubleClaim {
	sorted := append([]claimEvent(nil), claims...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].at.Before(sorted[j].at) })
	held := map[int64]claimEvent{}
	out := []DoubleClaim{}
	for _, cl := range sorted {
		prev, ok := held[cl.intent]
		if ok && prev.instance != cl.instance && cl.at.Before(prev.expires) &&
			!releasedBetween(releases, cl.intent, prev.at, cl.at) {
			out = append(out, DoubleClaim{
				IntentID: cl.intent, FirstInstance: prev.instance, FirstHolder: prev.holder, FirstExpires: prev.expires,
				SecondInstance: cl.instance, SecondHolder: cl.holder, SecondAt: cl.at,
			})
		}
		held[cl.intent] = cl
	}
	return out
}

// workTools are the ARTEX tools that record results or act on the target.
// Planning, reads and claim bookkeeping are not counted as work.
var workTools = map[string]bool{
	"report_finding": true, "record_fact": true, "record_coverage": true, "insert_assets": true,
	"save_evidence": true, "capture_command_output": true, "ingest_scanner_output": true, "ingest_masscan": true,
}

// unclaimedWork counts the instance's Bash calls (failed commands included)
// and successful workTools calls made while it held no live claim on any
// intent, out of WorkCalls such calls in all. Calls before the first intent
// of the run was created are skipped, since nothing could be claimed then.
// It also lists the intents the instance completed or canceled without ever
// having claimed them, except that canceling an unclaimed intent it created
// itself is listed apart as OwnCancels: that is the duplicate cleanup the
// server asks for when add_intent reports an overlapping open intent.
func unclaimedWork(t *Transcript, claims []claimEvent, releases []releaseEvent, nodes map[int64]nodeOrigin, firstIntent time.Time) UnclaimedWork {
	var u UnclaimedWork
	for _, call := range t.Calls {
		switch {
		case call.Tool == "complete_intent" || call.Tool == "cancel_intent":
			m := doneRe.FindStringSubmatch(call.Result)
			if m == nil || call.IsError || conflictRe.MatchString(call.Result) {
				continue
			}
			id, _ := strconv.ParseInt(m[1], 10, 64)
			if claimedBefore(claims, t.Instance, id, call.At) || slices.Contains(u.Finishes, id) || slices.Contains(u.OwnCancels, id) {
				continue
			}
			if n, ok := nodes[id]; ok && call.Tool == "cancel_intent" && n.kind == tools.KindIntent && n.instance == t.Instance {
				u.OwnCancels = append(u.OwnCancels, id)
			} else {
				u.Finishes = append(u.Finishes, id)
			}
		case call.Tool == "Bash" || workTools[call.Tool] && !call.IsError:
			if firstIntent.IsZero() || call.At.Before(firstIntent) {
				continue
			}
			u.WorkCalls++
			if holdsClaim(claims, releases, t.Instance, call.At) {
				continue
			}
			u.Calls++
			if call.Tool == "Bash" {
				u.Bash++
			} else {
				u.Records++
			}
		}
	}
	return u
}

func holdsClaim(claims []claimEvent, releases []releaseEvent, instance int, at time.Time) bool {
	for _, cl := range claims {
		if cl.instance == instance && !cl.at.After(at) && at.Before(cl.expires) && !releasedBetween(releases, cl.intent, cl.at, at) {
			return true
		}
	}
	return false
}

func claimedBefore(claims []claimEvent, instance int, intent int64, at time.Time) bool {
	for _, cl := range claims {
		if cl.instance == instance && cl.intent == intent && !cl.at.After(at) {
			return true
		}
	}
	return false
}

// recordedIntents returns the intent of each fact or finding a call recorded.
func recordedIntents(call ToolCall) []int64 {
	var ids []int64
	switch call.Tool {
	case "report_finding":
		ids = append(ids, inputIntentID(call.Input))
	case "record_fact":
		var a struct {
			Facts []struct {
				IntentID json.RawMessage `json:"intent_id"`
			} `json:"facts"`
		}
		_ = json.Unmarshal(call.Input, &a)
		for _, f := range a.Facts {
			ids = append(ids, rawID(f.IntentID))
		}
	}
	return slices.DeleteFunc(ids, func(id int64) bool { return id <= 0 })
}

// concurrentWork flags two instances whose work windows on one intent
// overlap. A window runs from the instance's first claim of the intent or
// first record against it, whichever is earlier, to its last record or its
// completion or cancellation, and needs at least one record or finish. Leases
// are ignored: this catches an instance that kept working after its lease
// lapsed and another instance took the intent.
func concurrentWork(ts []*Transcript, claims []claimEvent, releases []releaseEvent) []ConcurrentWork {
	type span struct {
		first, last time.Time
		n           int
		worked      bool
	}
	spans := map[int64]map[int]*span{}
	extend := func(intent int64, instance int, at time.Time, start bool) *span {
		if spans[intent] == nil {
			spans[intent] = map[int]*span{}
		}
		s := spans[intent][instance]
		if s == nil {
			s = &span{first: at, last: at}
			spans[intent][instance] = s
		}
		if at.Before(s.first) {
			s.first = at
		}
		if !start && (!s.worked || at.After(s.last)) {
			s.last = at
			s.worked = true
		}
		return s
	}
	for _, cl := range claims {
		extend(cl.intent, cl.instance, cl.at, true)
	}
	for _, r := range releases {
		extend(r.intent, r.instance, r.at, false)
	}
	for _, t := range ts {
		for _, call := range t.Calls {
			if call.IsError {
				continue
			}
			for _, id := range recordedIntents(call) {
				extend(id, t.Instance, call.At, false).n++
			}
		}
	}
	for _, byInst := range spans {
		for inst, s := range byInst {
			if !s.worked {
				delete(byInst, inst)
			}
		}
	}
	out := []ConcurrentWork{}
	for intent, byInst := range spans {
		var insts []int
		for inst := range byInst {
			insts = append(insts, inst)
		}
		sort.Ints(insts)
		for i, a := range insts {
			for _, b := range insts[i+1:] {
				sa, sb := byInst[a], byInst[b]
				if !sa.first.Before(sb.last) || !sb.first.Before(sa.last) {
					continue
				}
				from, to := sa.first, sa.last
				if sb.first.After(from) {
					from = sb.first
				}
				if sb.last.Before(to) {
					to = sb.last
				}
				out = append(out, ConcurrentWork{IntentID: intent, FirstInstance: a, FirstRecords: sa.n,
					SecondInstance: b, SecondRecords: sb.n, From: from, To: to})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IntentID != out[j].IntentID {
			return out[i].IntentID < out[j].IntentID
		}
		if out[i].FirstInstance != out[j].FirstInstance {
			return out[i].FirstInstance < out[j].FirstInstance
		}
		return out[i].SecondInstance < out[j].SecondInstance
	})
	return out
}

type createdNode struct {
	id        int64
	kind      string
	intent    int64
	findingID int64
}

func createdNodes(call ToolCall) []createdNode {
	text := strings.TrimSpace(call.Result)
	if call.Tool == "report_finding" {
		var r struct {
			FindingID int64 `json:"finding_id"`
			NodeID    int64 `json:"node_id"`
		}
		var a struct {
			IntentID json.RawMessage `json:"intent_id"`
		}
		_ = json.Unmarshal(call.Input, &a)
		if json.Unmarshal([]byte(text), &r) != nil || r.NodeID == 0 {
			return nil
		}
		return []createdNode{{r.NodeID, tools.KindFinding, rawID(a.IntentID), r.FindingID}}
	}

	kind := tools.KindIntent
	if call.Tool == "record_fact" {
		kind = tools.KindFact
	}
	var a struct {
		Facts []struct {
			IntentID json.RawMessage `json:"intent_id"`
		} `json:"facts"`
	}
	_ = json.Unmarshal(call.Input, &a)
	intentOf := func(i int) int64 {
		if kind == tools.KindFact && i < len(a.Facts) {
			return rawID(a.Facts[i].IntentID)
		}
		return 0
	}

	if m := createdRe.FindStringSubmatch(text); m != nil {
		id, _ := strconv.ParseInt(m[2], 10, 64)
		return []createdNode{{id, kind, intentOf(0), 0}}
	}
	var batch struct {
		IDs []int64 `json:"ids"`
	}
	if json.Unmarshal([]byte(text), &batch) != nil {
		return nil
	}
	var out []createdNode
	for i, id := range batch.IDs {
		if id > 0 {
			out = append(out, createdNode{id, kind, intentOf(i), 0})
		}
	}
	return out
}

func rawID(raw json.RawMessage) int64 {
	n, _ := strconv.ParseInt(strings.Trim(string(raw), `"`), 10, 64)
	return n
}

func normVulnClass(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// duplicateFindings groups findings by vulnclass and asset (0 for findings
// with no asset) and counts the extra findings in groups reported by more
// than one instance.
func duplicateFindings(findings []EngagementFinding, owner map[int64]int) DuplicateFindings {
	type key struct {
		vc    string
		asset int64
	}
	groups := map[key][]EngagementFinding{}
	var keys []key
	for _, f := range findings {
		assets := f.AssetIDs
		if len(assets) == 0 {
			assets = []int64{0}
		}
		for _, a := range assets {
			k := key{normVulnClass(f.VulnClass), a}
			if groups[k] == nil {
				keys = append(keys, k)
			}
			groups[k] = append(groups[k], f)
		}
	}

	var out DuplicateFindings
	for _, k := range keys {
		fs := groups[k]
		instSet := map[int]bool{}
		liveInst := map[int]bool{}
		live := 0
		g := DuplicateGroup{VulnClass: k.vc, AssetID: k.asset}
		for _, f := range fs {
			g.FindingIDs = append(g.FindingIDs, f.ID)
			inst, known := owner[f.ID]
			if known {
				instSet[inst] = true
			}
			if f.MergedInto == 0 && !f.Dismissed {
				live++
				if known {
					liveInst[inst] = true
				}
			}
		}
		if len(instSet) < 2 {
			continue
		}
		for inst := range instSet {
			g.Instances = append(g.Instances, inst)
		}
		sort.Ints(g.Instances)
		g.Live = live
		out.Raw += len(fs) - 1
		if len(liveInst) >= 2 {
			out.AfterMerges += live - 1
		}
		out.Groups = append(out.Groups, g)
	}
	return out
}

func redundantRecon(commands map[string]map[int]int) RedundantRecon {
	var out RedundantRecon
	for cmd, byInst := range commands {
		if len(byInst) < 2 {
			continue
		}
		sc := SharedCommand{Command: cmd}
		first := true
		for inst, n := range byInst {
			sc.Instances = append(sc.Instances, inst)
			sc.Runs += n
			if !first {
				out.RedundantRuns++
			}
			first = false
		}
		sort.Ints(sc.Instances)
		out.SharedCommands++
		out.Examples = append(out.Examples, sc)
	}
	sort.Slice(out.Examples, func(i, j int) bool {
		if len(out.Examples[i].Instances) != len(out.Examples[j].Instances) {
			return len(out.Examples[i].Instances) > len(out.Examples[j].Instances)
		}
		return out.Examples[i].Command < out.Examples[j].Command
	})
	if len(out.Examples) > 20 {
		out.Examples = out.Examples[:20]
	}
	return out
}

// takeovers finds the intents the killed instance was the last to hold going
// into its death and whether another instance took each one over afterwards. A
// candidate is an intent the killed instance claimed (with implicit renewals
// already replayed) and neither it nor anyone else released before the kill. An
// intent whose lease was still live at the kill is always a candidate. One
// whose lease had already lapsed is a candidate only when the killed instance
// was still active within a lease of the kill (lastActivity) and no other
// instance claimed it since the killed instance's last claim: that covers a
// holder still working over the shell when its lease lapsed shortly before the
// kill (shell and target-change work do not renew the claim), without reviving
// intents it had abandoned. A live-at-kill candidate nobody reclaimed is
// flagged as never taken over; a lapsed candidate is reported only when it was
// actually taken over.
func takeovers(killed int, killedAt, lastActivity time.Time, claims []claimEvent, releases []releaseEvent, nodes map[int64]nodeOrigin, edges []tools.Edge, lease time.Duration) []Takeover {
	last := map[int64]claimEvent{}
	for _, cl := range claims {
		if cl.instance == killed && !cl.at.After(killedAt) && cl.at.After(last[cl.intent].at) {
			last[cl.intent] = cl
		}
	}
	working := lastActivity.After(killedAt.Add(-lease))
	live := map[int64]bool{}
	var ids []int64
	for id, cl := range last {
		if releasedBetween(releases, id, cl.at, killedAt) {
			continue
		}
		if cl.expires.After(killedAt) {
			live[id] = true
		} else if !working || claimedByOtherBetween(claims, killed, id, cl.at, killedAt) {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var out []Takeover
	for _, id := range ids {
		tk := Takeover{IntentID: id, KilledInstance: killed, LeaseExpires: last[id].expires}
		for _, cl := range claims {
			if cl.intent == id && cl.instance != killed && !cl.implicit && cl.at.After(killedAt) && (tk.TakenAt == nil || cl.at.Before(*tk.TakenAt)) {
				at := cl.at
				tk.TakenBy, tk.TakenAt = cl.instance, &at
				tk.AfterLeaseLapsed = !at.Before(tk.LeaseExpires)
			}
		}
		if tk.TakenAt == nil && !live[id] {
			continue
		}
		attached := map[int64]bool{}
		for _, e := range edges {
			if e.From == id {
				attached[e.To] = true
			}
		}
		for nid, n := range nodes {
			if n.instance == killed && n.intent == id {
				tk.FactsRecorded++
				if attached[nid] {
					tk.FactsAttached++
				}
			}
		}
		out = append(out, tk)
	}
	return out
}

func loadTranscripts(runs []InstanceRun) ([]*Transcript, map[int]time.Time, error) {
	var ts []*Transcript
	kills := map[int]time.Time{}
	for _, r := range runs {
		t, err := parseTranscript(r.Instance, r.Transcript)
		if err != nil {
			return nil, nil, fmt.Errorf("transcript %d: %w", r.Instance, err)
		}
		ts = append(ts, t)
		if r.KilledAt != "" {
			if at, err := time.Parse(time.RFC3339Nano, r.KilledAt); err == nil {
				kills[r.Instance] = at
			}
		}
	}
	return ts, kills, nil
}

func loadEngagement(ctx context.Context, s *store.Store, explorationID int64) ([]EngagementFinding, []tools.Edge, error) {
	list, err := s.ListFindings(ctx, explorationID)
	if err != nil {
		return nil, nil, err
	}
	var findings []EngagementFinding
	for _, f := range list {
		ef := EngagementFinding{ID: f.ID, VulnClass: f.VulnClass, MergedInto: f.MergedInto, Dismissed: f.Dismissed}
		if full, err := s.GetFinding(f.ID); err == nil && full != nil && len(full.AssetIDs) > 0 {
			_ = json.Unmarshal(full.AssetIDs, &ef.AssetIDs)
		}
		findings = append(findings, ef)
	}
	edges, err := s.Edges(ctx, explorationID, 0)
	if err != nil {
		return nil, nil, err
	}
	return findings, edges, nil
}

func (c *Coordination) Summary() string {
	var b strings.Builder
	pass := func(ok bool) string {
		if ok {
			return "pass"
		}
		return "FAIL"
	}
	fmt.Fprintf(&b, "Coordination scorecard\n\n")
	if c.ImplicitRenewal {
		fmt.Fprintf(&b, "Claim replay: start_intent plus %d implicit renewals from recorded work\n", c.ImplicitRenewals)
	} else {
		fmt.Fprintf(&b, "Claim replay: start_intent only (server without implicit renewal)\n")
	}
	fmt.Fprintf(&b, "Concurrent double claims: %d (%s)\n", len(c.DoubleClaims), pass(len(c.DoubleClaims) == 0))
	for _, d := range c.DoubleClaims {
		fmt.Fprintf(&b, "  intent %d: instance %d (%s) then instance %d (%s) at %s, before %s\n",
			d.IntentID, d.FirstInstance, d.FirstHolder, d.SecondInstance, d.SecondHolder,
			d.SecondAt.Format(time.RFC3339), d.FirstExpires.Format(time.RFC3339))
	}
	fmt.Fprintf(&b, "Concurrent work on one intent: %d (%s)\n", len(c.ConcurrentWork), pass(len(c.ConcurrentWork) == 0))
	for _, w := range c.ConcurrentWork {
		fmt.Fprintf(&b, "  intent %d: instance %d (%d records) and instance %d (%d records) interleaved from %s to %s\n",
			w.IntentID, w.FirstInstance, w.FirstRecords, w.SecondInstance, w.SecondRecords,
			w.From.Format(time.RFC3339), w.To.Format(time.RFC3339))
	}
	fmt.Fprintf(&b, "Work outside a claimed intent: %d calls, %d unclaimed intents completed or canceled, %d own duplicates canceled\n",
		c.UnclaimedWork, c.UnclaimedFinishes, c.OwnCancels)
	for _, i := range c.Instances {
		if u := i.Unclaimed; u.Calls > 0 || len(u.Finishes) > 0 || len(u.OwnCancels) > 0 {
			fmt.Fprintf(&b, "  instance %d: %d of %d work calls (%d bash, %d artex records)", i.Instance, u.Calls, u.WorkCalls, u.Bash, u.Records)
			if len(u.Finishes) > 0 {
				fmt.Fprintf(&b, ", finished unclaimed intents %v", u.Finishes)
			}
			if len(u.OwnCancels) > 0 {
				fmt.Fprintf(&b, ", canceled own duplicates %v", u.OwnCancels)
			}
			b.WriteString("\n")
		}
	}
	if c.Conflicts > 0 {
		fmt.Fprintf(&b, "Conflict responses handled: %d/%d (%.0f%%, %s)\n", c.ConflictsHandled, c.Conflicts,
			c.ConflictsHandledPc, pass(c.ConflictsHandledPc >= 90))
	} else {
		fmt.Fprintf(&b, "Conflict responses handled: no conflicts\n")
	}
	for _, u := range c.UnhandledConflicts {
		fmt.Fprintf(&b, "  unhandled: instance %d call %d on intent %d held by %s", u.Instance, u.Call, u.IntentID, u.Holder)
		if u.HolderInstance > 0 {
			fmt.Fprintf(&b, " (instance %d)", u.HolderInstance)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Refused completions: %d\n", c.RefusedCompletions)
	if c.HolderMismatches > 0 {
		fmt.Fprintf(&b, "Claims under another instance's identity: %d (FAIL)\n", c.HolderMismatches)
	}
	fmt.Fprintf(&b, "Duplicate findings: %d raw, %d after merges (%s)\n", c.DuplicateFindings.Raw,
		c.DuplicateFindings.AfterMerges, pass(c.DuplicateFindings.AfterMerges == 0))
	for _, g := range c.DuplicateFindings.Groups {
		fmt.Fprintf(&b, "  %s on asset %d: findings %v from instances %v, %d live\n", g.VulnClass, g.AssetID, g.FindingIDs, g.Instances, g.Live)
	}
	fmt.Fprintf(&b, "Redundant recon: %d commands run by more than one instance, %d extra runs\n",
		c.RedundantRecon.SharedCommands, c.RedundantRecon.RedundantRuns)
	for _, e := range c.RedundantRecon.Examples {
		fmt.Fprintf(&b, "  %v: %s\n", e.Instances, e.Command)
	}
	fmt.Fprintf(&b, "Cross-instance parents: %d %v\n", c.CrossParents.Total, c.CrossParents.ByKind)
	for _, tk := range c.Takeovers {
		if tk.TakenAt == nil {
			fmt.Fprintf(&b, "Takeover: intent %d held by killed instance %d was never taken over (FAIL)\n", tk.IntentID, tk.KilledInstance)
			continue
		}
		fmt.Fprintf(&b, "Takeover: intent %d from instance %d taken by instance %d at %s (lease lapsed: %t), facts attached %d/%d\n",
			tk.IntentID, tk.KilledInstance, tk.TakenBy, tk.TakenAt.Format(time.RFC3339), tk.AfterLeaseLapsed, tk.FactsAttached, tk.FactsRecorded)
	}
	fmt.Fprintf(&b, "\n%-9s %6s %5s %12s %7s %9s %6s %7s %10s %9s %9s\n",
		"instance", "calls", "bash", "list_changes", "claims", "conflicts", "nodes", "x-par", "in+cache", "out", "cost")
	for _, i := range c.Instances {
		fmt.Fprintf(&b, "%-9d %6d %5d %12d %7d %9d %6d %7d %10d %9d %9.2f\n",
			i.Instance, i.ToolCalls, i.BashCommands, i.ListChanges, i.ClaimsTaken, i.Conflicts, i.NodesCreated, i.CrossParents,
			i.Usage.InputTokens+i.Usage.CacheCreationTokens+i.Usage.CacheReadTokens, i.Usage.OutputTokens, i.CostUSD)
	}
	fmt.Fprintf(&b, "total: %d input+cache tokens, %d output tokens, $%.2f\n",
		c.Total.Usage.InputTokens+c.Total.Usage.CacheCreationTokens+c.Total.Usage.CacheReadTokens,
		c.Total.Usage.OutputTokens, c.Total.CostUSD)
	return b.String()
}

func scoreRun(ctx context.Context, s *store.Store, explorationID int64, runs []InstanceRun, lease time.Duration, renewal bool) (*Coordination, error) {
	ts, kills, err := loadTranscripts(runs)
	if err != nil {
		return nil, err
	}
	findings, edges, err := loadEngagement(ctx, s, explorationID)
	if err != nil {
		return nil, err
	}
	return scoreCoordination(CoordinationInput{Transcripts: ts, Findings: findings, Edges: edges, Kills: kills, Lease: lease, Renewal: renewal}), nil
}

// serverRenewsClaims reports whether the artex-cc binary's start_intent says
// recorded work renews the claim. Binaries without the tools command, or
// that fail to run, count as not renewing.
func serverRenewsClaims(ctx context.Context, artexBin string) bool {
	out, err := exec.CommandContext(ctx, artexBin, "tools", "--json").Output()
	if err != nil {
		return false
	}
	var catalog []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if json.Unmarshal(out, &catalog) != nil {
		return false
	}
	for _, e := range catalog {
		if e.Name == "start_intent" {
			return strings.Contains(e.Description, "renews whenever you pass this intent as intent_id")
		}
	}
	return false
}
