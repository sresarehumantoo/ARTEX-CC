package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func RegisterExplorationTools(reg *Registry, store Store, explorationID int64) {
	reg.Register(makeGraphOverview(store, explorationID))
	reg.Register(makeAddIntent(store, explorationID))
	reg.Register(makeRecordFact(store, explorationID))
	reg.Register(makeListFacts(store, explorationID))
	reg.Register(makeNodeDetail(store))
	reg.Register(makeAddHint(store, explorationID))
	reg.Register(makeSetGoals(store, explorationID))
	reg.Register(makeSetConstraints(store, explorationID))
	reg.Register(makeProveGoal(store, explorationID))
}

func makeGraphOverview(store Store, explorationID int64) Tool {
	return Tool{
		Name:     "graph_overview",
		Category: CategoryExploration,
		Description: "The task board: goals and their states, open/running/done intents, recent facts, findings, credentials, coverage, and unrestored_target_changes (see record_target_change). " +
			"Call this FIRST at the start of every planning cycle to understand what has been done, what is in progress, and what needs doing next. " +
			"After subagents complete work, call it again to see their results and plan the next round. " +
			"Includes strategic hints from the human/main agent that must be incorporated when generating intents.",
		ReadOnly: true,
		Schema:   obj(map[string]any{}),
		Run: func(ctx context.Context, _ json.RawMessage) (Result, error) {
			ov, err := store.GraphOverview(ctx, explorationID)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(ov)
		},
	}
}

func makeAddIntent(store Store, explorationID int64) Tool {
	return Tool{
		Name:     "add_intent",
		Category: CategoryExploration,
		Description: "Generate exploration directions into the frontier and link them into the exploration graph. " +
			"An intent is an open exploration direction: describe freely in one summary sentence what to explore/verify/exploit.\n" +
			"Batch preferred: put multiple new directions into the intents array in a single submission (saves round trips). " +
			"Returns an ids array, same length and order as intents. For a single intent, omit intents and pass the top-level summary.\n" +
			"After calling graph_overview and identifying gaps, use this to queue new work. Each intent is one exploration direction.\n" +
			"Other agents may plan the same work at the same time. An intent whose summary (ignoring case, punctuation and spacing) and asset_ids " +
			"match an open or running intent is not added: skipped gives the existing_id to start_intent instead. " +
			"open_intents lists unclaimed intents that already existed; if one covers the same work as an intent you added, work that one and cancel_intent yours.",
		Schema: obj(map[string]any{
			"intents":    arr(map[string]any{"type": "object"}, "[Prefer this] array of exploration directions to add"),
			"summary":    str("[single] one sentence describing this exploration direction"),
			"asset_ids":  arr(map[string]any{"type": "integer"}, "target asset ids this direction tests/attacks"),
			"parent_ids": arr(map[string]any{"type": "integer"}, "upstream anchor ids: which confirmed fact/finding nodes this direction is synthesized from"),
			"priority":   intp("priority 0-10, default 5"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			type intentItem struct {
				Summary   string            `json:"summary"`
				AssetIDs  []json.RawMessage `json:"asset_ids"`
				ParentIDs []json.RawMessage `json:"parent_ids"`
				Priority  int               `json:"priority"`
			}
			var a struct {
				Intents []intentItem `json:"intents"`
				intentItem
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			items, isBatch := batchOrSingle(a.Intents, a.intentItem)
			if len(items) > maxBatchLen {
				return errResult(fmt.Sprintf("intents array too large (%d items, max %d)", len(items), maxBatchLen)), nil
			}

			checkParents := func(parents []int64) error {
				for _, p := range parents {
					n, err := store.NodeDetail(ctx, p)
					if err != nil || n == nil {
						return fmt.Errorf("parent_id %d does not exist", p)
					}
					if n.Kind != KindFact && n.Kind != KindFinding {
						return fmt.Errorf("parent_id %d is a %q node: intents can only anchor on fact/finding nodes", p, n.Kind)
					}
				}
				return nil
			}

			errs := map[string]string{}
			var inputs []IntentInput
			var itemIndex []int
			for i, it := range items {
				if strings.TrimSpace(it.Summary) == "" {
					errs[strconv.Itoa(i)] = "summary must not be empty"
					continue
				}
				parents := pidList(it.ParentIDs)
				if err := checkParents(parents); err != nil {
					errs[strconv.Itoa(i)] = err.Error()
					continue
				}
				priority := it.Priority
				if priority == 0 {
					priority = 5
				}
				inputs = append(inputs, IntentInput{Summary: it.Summary, Priority: priority, AssetIDs: pidList(it.AssetIDs), ParentIDs: parents})
				itemIndex = append(itemIndex, i)
			}

			ids := make([]int64, len(items))
			var skipped []SkippedIntent
			var open []IntentRef
			if len(inputs) > 0 {
				res, err := store.AddIntents(ctx, explorationID, inputs)
				if err != nil {
					return errResult(err.Error()), nil
				}
				for j, id := range res.IDs {
					ids[itemIndex[j]] = id
				}
				for j, e := range res.Errors {
					errs[strconv.Itoa(itemIndex[j])] = e
				}
				for _, sk := range res.Skipped {
					sk.Index = itemIndex[sk.Index]
					skipped = append(skipped, sk)
				}
				open = res.Open
			}

			if !isBatch {
				if e, bad := errs["0"]; bad {
					return errResult(e), nil
				}
				if len(skipped) == 0 && len(open) == 0 {
					return textResult(fmt.Sprintf("intent created: %d", ids[0])), nil
				}
			}
			out := map[string]any{"ids": ids}
			if len(errs) > 0 {
				out["errors"] = errs
			}
			var notes []string
			if len(skipped) > 0 {
				out["skipped"] = skipped
				notes = append(notes, "Skipped intents match an open or running intent (existing_id) and were not added: "+
					"start_intent that one instead unless another session holds it.")
			}
			if len(open) > 0 {
				out["open_intents"] = open
				notes = append(notes, "open_intents already existed and nobody holds them, possibly planned by another agent: "+
					"if one covers the same work as an intent you just added, work that one and cancel_intent yours.")
			}
			if len(notes) > 0 {
				out["note"] = strings.Join(notes, " ")
			}
			return jsonResult(out)
		},
	}
}

func makeRecordFact(store Store, explorationID int64) Tool {
	return Tool{
		Name:     "record_fact",
		Category: CategoryExploration,
		Description: "Write an exploration fact/conclusion into the exploration graph, linked to the intent that produced it (intent_id). " +
			"Passing intent_id for an intent you hold also renews your claim on it (pass owner too if you claimed with one). " +
			"Call this IMMEDIATELY when you learn something, do not wait until the end. " +
			"Facts written here are visible to the coordinator and other subagents via graph_overview and list_facts.\n" +
			"Used to record both positive conclusions (fingerprinting/enumeration) and negative conclusions ('port closed'/'parameter not injectable').\n" +
			"Multiple observations from one exploration must be consolidated into ONE fact, not split into many.\n" +
			"facts is the preferred parameter: facts:[{...}] with one item per mutually distinct conclusion.\n" +
			"evidence = one line of key evidence supporting this conclusion.\n" +
			"confidence = observed (seen directly) | inferred (inferred from phenomena).\n" +
			"source = target_response | tool_output | third_party_intel | operator_input | inference (default).",
		Schema: obj(map[string]any{
			"facts": map[string]any{
				"type":        "array",
				"minItems":    1,
				"description": "The facts to record, usually exactly one.",
				"items": obj(map[string]any{
					"summary":    str("Required: a non-empty one-sentence summary of this conclusion."),
					"intent_id":  intp("The intent that produced this fact."),
					"detail":     str("Details of this fact."),
					"evidence":   str("One line of key evidence."),
					"confidence": str("observed | inferred"),
					"source":     enm([]string{"target_response", "tool_output", "third_party_intel", "operator_input", "inference"}, "where the content came from"),
					"asset_ids":  arr(map[string]any{"type": "integer"}, "Asset IDs explicitly involved in this fact."),
				}, "summary"),
			},
			"owner": str(renewOwnerArgDesc),
		}, "facts"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			type factItem struct {
				Summary    string            `json:"summary"`
				Detail     string            `json:"detail"`
				Evidence   string            `json:"evidence"`
				Confidence string            `json:"confidence"`
				Source     string            `json:"source"`
				IntentID   json.RawMessage   `json:"intent_id"`
				AssetIDs   []json.RawMessage `json:"asset_ids"`
			}
			var a struct {
				Facts []factItem `json:"facts"`
				Owner string     `json:"owner"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if len(a.Facts) == 0 {
				return errResult("facts must be a non-empty array"), nil
			}

			var worked []int64
			recordOne := func(it factItem) (int64, error) {
				if strings.TrimSpace(it.Summary) == "" {
					return 0, fmt.Errorf("summary must not be empty")
				}
				source := strings.ToLower(strings.TrimSpace(it.Source))
				if source == "" {
					source = "inference"
				}
				validSources := map[string]bool{"target_response": true, "tool_output": true, "third_party_intel": true, "operator_input": true, "inference": true}
				if !validSources[source] {
					return 0, fmt.Errorf("source %q is not valid", it.Source)
				}
				anchors := pidList(it.AssetIDs)
				payload := map[string]any{"summary": it.Summary, "source": source}
				if it.Detail != "" {
					payload["detail"] = it.Detail
				}
				if e := strings.TrimSpace(it.Evidence); e != "" {
					payload["evidence"] = e
				}
				if c := strings.TrimSpace(it.Confidence); c != "" {
					payload["confidence"] = c
				}

				intent := pid(it.IntentID)
				if intent > 0 {
					node, err := store.NodeDetail(ctx, intent)
					if err != nil || node == nil || node.Kind != KindIntent {
						return 0, fmt.Errorf("intent_id must be an intent of this task")
					}
				}

				id, err := store.AddNode(ctx, explorationID, KindFact, payload, 5, "confirmed", "worker", anchors)
				if err != nil {
					return 0, err
				}
				if intent > 0 {
					_ = store.AddEdge(ctx, intent, RelYields, id)
					worked = append(worked, intent)
				}
				return id, nil
			}

			ids, errs := runBatch(a.Facts, recordOne)
			renewHeldIntents(ctx, store, a.Owner, worked)
			isBatch := len(a.Facts) > 1
			return singleOrBatchReturn(ids, errs, isBatch, "fact")
		},
	}
}

func makeListFacts(store Store, explorationID int64) Tool {
	return Tool{
		Name:     "list_facts",
		Category: CategoryExploration,
		Description: "Page through the exploration facts/conclusions, newest first (compact: id+summary+state+origin+source). " +
			"All parameters optional: limit (default 20, max 100). Returns {facts, total}.",
		ReadOnly: true,
		Schema: obj(map[string]any{
			"limit": intp("number of entries to return, default 20, max 100"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Limit int `json:"limit"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if a.Limit <= 0 {
				a.Limit = 20
			}
			if a.Limit > 100 {
				a.Limit = 100
			}
			nodes, err := store.ListNodes(ctx, explorationID, KindFact, a.Limit)
			if err != nil {
				return errResult(err.Error()), nil
			}
			out := make([]map[string]any, 0, len(nodes))
			for _, n := range nodes {
				m := map[string]any{"id": n.ID, "state": n.State}
				if n.Payload != nil {
					m["summary"] = truncRunes(fmt.Sprint(n.Payload["summary"]), 160)
				}
				if n.Origin != "" {
					m["origin"] = n.Origin
				}
				if s, ok := n.Payload["source"].(string); ok && s != "" {
					m["source"] = s
				}
				out = append(out, m)
			}
			return jsonResult(map[string]any{"facts": out, "total": len(out)})
		},
	}
}

func makeNodeDetail(store Store) Tool {
	return Tool{
		Name:     "node_detail",
		Category: CategoryExploration,
		Description: "Fetch the full content of an exploration graph node by id. " +
			"Only for exploration node ids returned by list_facts/list_findings/graph_overview; for assets use list_assets.",
		ReadOnly: true,
		Schema:   obj(map[string]any{"id": intp("exploration graph node id")}, "id"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				ID json.RawMessage `json:"id"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			id := pid(a.ID)
			if id <= 0 {
				return errResult("id is required"), nil
			}
			n, err := store.NodeDetail(ctx, id)
			if err != nil {
				return errResult(err.Error()), nil
			}
			if n == nil {
				return errResult(fmt.Sprintf("exploration node %d not found", id)), nil
			}
			return jsonResult(n)
		},
	}
}

func makeAddHint(store Store, explorationID int64) Tool {
	return Tool{
		Name:     "add_hint",
		Category: CategoryExploration,
		Description: "Attach a strategic hint to the exploration graph; the planner reads it the next time it generates intents.\n" +
			"Batch preferred: put multiple hints in the hints array in a single submission. " +
			"Returns an ids array. For a single hint, omit hints and pass the top-level text.",
		Schema: obj(map[string]any{
			"hints":     arr(obj(map[string]any{"text": str("hint content"), "asset_ids": arr(map[string]any{"type": "integer"}, "asset ids to anchor (optional)")}, "text"), "[Prefer this] array of hints to add"),
			"text":      str("[single] hint content"),
			"asset_ids": arr(map[string]any{"type": "integer"}, "asset ids to anchor (optional)"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			type hintItem struct {
				Text     string            `json:"text"`
				AssetIDs []json.RawMessage `json:"asset_ids"`
			}
			var a struct {
				Hints []hintItem `json:"hints"`
				hintItem
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			items, isBatch := batchOrSingle(a.Hints, a.hintItem)
			if len(items) > maxBatchLen {
				return errResult(fmt.Sprintf("hints array too large (%d items, max %d)", len(items), maxBatchLen)), nil
			}

			addOne := func(it hintItem) (int64, error) {
				if strings.TrimSpace(it.Text) == "" {
					return 0, fmt.Errorf("text must not be empty")
				}
				return AddHint(ctx, store, explorationID, it.Text, "human", pidList(it.AssetIDs))
			}

			ids, errs := runBatch(items, addOne)
			return singleOrBatchReturn(ids, errs, isBatch, "hint")
		},
	}
}

// AddHint records a hint node in the state the planner reads.
func AddHint(ctx context.Context, store Store, explorationID int64, text, origin string, anchors []int64) (int64, error) {
	return store.AddNode(ctx, explorationID, KindHint, map[string]any{"text": text}, 0, StateHintActive, origin, anchors)
}

func makeSetGoals(store Store, explorationID int64) Tool {
	return Tool{
		Name:     "set_goals",
		Category: CategoryExploration,
		Description: "Set the exploration goals of this task. A goal = a final deliverable/verifiable result, not an attack step.\n" +
			"Batch preferred: put multiple goals in the goals array. For a single goal, pass the top-level text.\n" +
			"vulnclass is optional: the corresponding vulnerability class.\n" +
			"Goals are set once per engagement. If the engagement already has goals (another agent may have set them), nothing is added and " +
			"existing_goals lists them: work toward those. Pass append: true only to add goals to an engagement that already has some. " +
			"A goal whose text matches an existing goal (ignoring case and whitespace) is always skipped.",
		Schema: obj(map[string]any{
			"goals":     arr(map[string]any{"type": "object"}, "[Prefer this] array of goals to add"),
			"text":      str("[single] one independently verifiable final goal"),
			"vulnclass": str("corresponding vulnerability class (optional)"),
			"append":    map[string]any{"type": "boolean", "description": "add these goals even though the engagement already has goals"},
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			type goalItem struct {
				Text      string `json:"text"`
				VulnClass string `json:"vulnclass"`
			}
			var a struct {
				Goals  []goalItem `json:"goals"`
				Append bool       `json:"append"`
				goalItem
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			items, isBatch := batchOrSingle(a.Goals, a.goalItem)
			if len(items) > maxBatchLen {
				return errResult(fmt.Sprintf("goals array too large (%d items, max %d)", len(items), maxBatchLen)), nil
			}

			errs := map[string]string{}
			var goals []GoalInput
			for i, it := range items {
				text := strings.TrimSpace(it.Text)
				if text == "" {
					errs[strconv.Itoa(i)] = "text must not be empty"
					continue
				}
				goals = append(goals, GoalInput{Text: text, VulnClass: strings.TrimSpace(it.VulnClass)})
			}
			if len(goals) == 0 {
				if !isBatch {
					return errResult("text must not be empty"), nil
				}
				return jsonResult(map[string]any{"created": []GoalRef{}, "errors": errs})
			}

			res, err := store.AddGoals(ctx, explorationID, goals, a.Append)
			if err != nil {
				return errResult(err.Error()), nil
			}
			out := map[string]any{"created": res.Created}
			if len(res.Skipped) > 0 {
				out["skipped"] = res.Skipped
			}
			if len(res.Existing) > 0 {
				out["existing_goals"] = res.Existing
				out["note"] = "This engagement already has goals, possibly set by another agent, so none were added. " +
					"If these duplicate existing_goals, work toward those. If you are adding a genuinely new objective, call set_goals again with append: true."
			} else if len(res.Skipped) > 0 {
				out["note"] = "Skipped goals match an existing goal (existing_id) ignoring case and whitespace."
			}
			if len(errs) > 0 {
				out["errors"] = errs
			}
			return jsonResult(out)
		},
	}
}

func makeSetConstraints(store Store, explorationID int64) Tool {
	return Tool{
		Name:     "set_constraints",
		Category: CategoryExploration,
		Description: "Add operation constraints to this task to bound exploration: type=allow (operations permitted) or deny (operations forbidden).\n" +
			"A constraint = a rule about which operations may/may not be performed.\n" +
			"Batch preferred: put multiple constraints in the constraints array. For a single constraint, pass top-level text/type.",
		Schema: obj(map[string]any{
			"constraints": arr(map[string]any{"type": "object"}, "[Prefer this] array of constraints to add"),
			"text":        str("[single] the content of one operation constraint"),
			"type":        str("allow or deny; defaults to deny when omitted"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			type constraintItem struct {
				Text string `json:"text"`
				Type string `json:"type"`
			}
			var a struct {
				Constraints []constraintItem `json:"constraints"`
				constraintItem
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			items, isBatch := batchOrSingle(a.Constraints, a.constraintItem)
			if len(items) > maxBatchLen {
				return errResult(fmt.Sprintf("constraints array too large (%d items, max %d)", len(items), maxBatchLen)), nil
			}

			addOne := func(it constraintItem) (int64, error) {
				text := strings.TrimSpace(it.Text)
				if text == "" {
					return 0, fmt.Errorf("text must not be empty")
				}
				kind := strings.TrimSpace(strings.ToLower(it.Type))
				if kind == "" {
					kind = "deny"
				}
				if kind != "allow" && kind != "deny" {
					return 0, fmt.Errorf("type must be allow or deny")
				}
				return store.AddConstraint(ctx, explorationID, kind, text, "agent")
			}

			ids, errs := runBatch(items, addOne)
			return singleOrBatchReturn(ids, errs, isBatch, "constraint")
		},
	}
}

func makeProveGoal(store Store, explorationID int64) Tool {
	return Tool{
		Name:     "prove_goal",
		Category: CategoryExploration,
		Description: "Call when you judge that a finding/fact proves a goal met: links the evidence node to the goal node and marks the goal met. " +
			"A fact counts as evidence only when its confidence is observed. " +
			"When evidence (findings or facts) proves a goal met, call this. When all goals are proved, the engagement is complete.",
		Schema: obj(map[string]any{
			"goal_id":     intp("goal node id"),
			"evidence_id": intp("id of the finding/fact node that proves it"),
			"reason":      str("why this evidence satisfies the goal"),
		}, "goal_id", "evidence_id"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				GoalID     json.RawMessage `json:"goal_id"`
				EvidenceID json.RawMessage `json:"evidence_id"`
				Reason     string          `json:"reason"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			goal, ev := pid(a.GoalID), pid(a.EvidenceID)
			if goal == 0 || ev == 0 {
				return errResult("goal_id and evidence_id are required"), nil
			}

			goalNode, err := store.NodeDetail(ctx, goal)
			if err != nil || goalNode == nil || goalNode.Kind != KindGoal {
				return errResult("goal_id must be a goal node"), nil
			}

			evidenceNode, err := store.NodeDetail(ctx, ev)
			if err != nil || evidenceNode == nil || (evidenceNode.Kind != KindFact && evidenceNode.Kind != KindFinding) {
				return errResult("evidence_id must be a fact/finding node"), nil
			}

			if evidenceNode.Kind == KindFact {
				confidence, _ := evidenceNode.Payload["confidence"].(string)
				if strings.ToLower(strings.TrimSpace(confidence)) != "observed" {
					if confidence == "" {
						confidence = "none"
					}
					return errResult(fmt.Sprintf("prove_goal refused: fact %d has confidence %q; a fact proves a goal only when its confidence is \"observed\"", ev, confidence)), nil
				}
			}

			if err := store.ProveGoal(ctx, goal, ev); err != nil {
				if errors.Is(err, ErrDeadEvidence) {
					return errResult(fmt.Sprintf("prove_goal refused: node %d is dismissed or merged; prove the goal with a live finding or fact", ev)), nil
				}
				return errResult("prove_goal failed: " + err.Error()), nil
			}

			goals, err := store.ListNodes(ctx, explorationID, KindGoal, 1000)
			if err == nil && len(goals) > 0 {
				allMet := true
				for _, g := range goals {
					if g.State != "met" {
						allMet = false
						break
					}
				}
				if allMet {
					return textResult(fmt.Sprintf("goal %d marked met; all goals of this task are met", goal)), nil
				}
			}
			return textResult(fmt.Sprintf("goal %d marked met", goal)), nil
		},
	}
}
