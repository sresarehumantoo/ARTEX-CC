package tools

import (
	"context"
	"encoding/json"
)

func RegisterCredentialTools(reg *Registry, store Store, taskID int64) {
	reg.Register(makeSaveTaskCredential(store, taskID))
	reg.Register(makeListTaskCredentials(store, taskID))
}

func makeSaveTaskCredential(store Store, taskID int64) Tool {
	return Tool{
		Name:     "save_task_credential",
		Category: CategoryCredentials,
		Description: "Record (or update) a test account for this task's target. ALWAYS call this immediately after registering an account on the target, " +
			"with the exact username and password you chose. Same (domain, username) is overwritten in place, not duplicated.\n" +
			"Returns: {id, domain, username, source}.",
		Schema: obj(map[string]any{
			"domain":   str("domain or host the account belongs to"),
			"username": str("login name / email used"),
			"password": str("the exact password that was set"),
			"note":     str("optional context"),
		}, "domain", "username", "password"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Domain   string `json:"domain"`
				Username string `json:"username"`
				Password string `json:"password"`
				Note     string `json:"note"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if taskID <= 0 {
				return errResult("save_task_credential is only available inside a task"), nil
			}
			id, err := store.SaveCredential(ctx, taskID, CredentialInput{
				Domain:   a.Domain,
				Username: a.Username,
				Password: a.Password,
				Source:   "agent",
				Note:     a.Note,
			})
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"id": id, "domain": a.Domain, "username": a.Username, "source": "agent",
			})
		},
	}
}

func makeListTaskCredentials(store Store, taskID int64) Tool {
	return Tool{
		Name:     "list_task_credentials",
		Category: CategoryCredentials,
		Description: "List this task's test accounts: user-declared accounts plus accounts recorded by earlier runs. " +
			"Check this BEFORE registering a new account, and use matching entries for authenticated testing.",
		ReadOnly: true,
		Schema:   obj(map[string]any{}),
		Run: func(ctx context.Context, _ json.RawMessage) (Result, error) {
			if taskID <= 0 {
				return errResult("list_task_credentials is only available inside a task"), nil
			}
			list, err := store.ListCredentials(ctx, taskID)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(map[string]any{"credentials": list})
		},
	}
}
