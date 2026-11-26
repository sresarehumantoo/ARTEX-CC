package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func RegisterAssetTools(reg *Registry, store Store, taskID int64) {
	reg.Register(makeInsertAssets(store, taskID))
	reg.Register(makeListAssets(store, taskID))
	reg.Register(makeListCompanies(store))
	reg.Register(makeAddCompanyScope(store))
	reg.Register(makeAddTaskScope(store, taskID))
	reg.Register(makeListUntestedAssets(store, taskID))
}

func makeInsertAssets(store Store, taskID int64) Tool {
	return Tool{
		Name:     "insert_assets",
		Category: CategoryAssets,
		Description: "Register newly discovered assets in bulk; multiple types can be mixed in one call (see the type enum).\n" +
			"Required fields per type: root_domain->domain; ip->ip (must be IPv4/IPv6, not a hostname); subdomain->domain; app->app_name; service(HTTP)->url; service(non-HTTP)->service_name+port (at least one of ip/domain); endpoint->url+method. See each field's own description for the rest.\n" +
			"auth/technologies/params are append-merged, not overwritten.\n" +
			"Returns: {results:[{index,id,type}], errors:[{index,error}]}",
		Schema: obj(map[string]any{
			"assets": map[string]any{
				"type":        "array",
				"description": "asset array; each element is one asset record",
				"items": obj(map[string]any{
					"type": map[string]any{
						"type":        "string",
						"enum":        []string{"root_domain", "ip", "subdomain", "app", "service", "endpoint"},
						"description": "asset type",
					},
					"domain":        str("root domain or subdomain (required for root_domain/subdomain)"),
					"icp":           str("ICP registration number (optional)"),
					"record_type":   str("DNS record type: A/AAAA/CNAME/MX etc. (optional for subdomain)"),
					"record_value":  arr(map[string]any{"type": "string"}, "list of DNS record values (optional for subdomain)"),
					"ip":            str("IP address (must be IPv4/IPv6, not a hostname; required for ip type)"),
					"bound_domains": arr(map[string]any{"type": "string"}, "list of domains bound to this IP (optional for ip type)"),
					"open_ports": map[string]any{
						"type":        "array",
						"description": "list of open ports (optional for ip type)",
						"items":       obj(map[string]any{"port": intp("port number"), "service": str("service name (optional)")}, "port"),
					},
					"app_name":       str("application name (required for app type)"),
					"bundle_id":      str("Bundle ID (optional for app type)"),
					"category":       str("application category (optional)"),
					"description":    str("application description (optional)"),
					"app_icp":        str("application ICP registration (optional)"),
					"company_id":     intp("owning company id (optional for app type)"),
					"url":            str("full URL including scheme and port (required for HTTP service)"),
					"status_code":    intp("HTTP response status code (optional)"),
					"content_length": intp("HTTP response body size in bytes (optional)"),
					"page_title":     str("page <title> content (optional)"),
					"favicon_mmh3":   str("favicon MMH3 hash (optional)"),
					"technologies":   arr(map[string]any{"type": "string"}, "tech-stack list (optional)"),
					"auth":           map[string]any{"type": "array", "description": "discovered credentials (optional)", "items": map[string]any{"type": "object"}},
					"service_name":   str("service name, e.g. ssh/mysql/redis (required for non-HTTP service)"),
					"service_ip":     str("optional enrichment IP"),
					"port":           intp("port number (required for non-HTTP service)"),
					"method":         str("HTTP method (required for endpoint)"),
					"params":         map[string]any{"type": "array", "description": "request parameters (optional)", "items": map[string]any{"type": "object"}},
				}, "type"),
			},
		}, "assets"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Assets []AssetInput `json:"assets"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if len(a.Assets) > maxBatchLen {
				return errResult(fmt.Sprintf("assets array too large (%d items, max %d)", len(a.Assets), maxBatchLen)), nil
			}
			if len(a.Assets) == 0 {
				return errResult("assets array must not be empty"), nil
			}

			type result struct {
				Index int    `json:"index"`
				ID    int64  `json:"id"`
				Type  string `json:"type"`
			}
			type errEntry struct {
				Index int    `json:"index"`
				Error string `json:"error"`
			}

			var results []result
			var errs []errEntry

			for i, item := range a.Assets {
				typ := strings.TrimSpace(item.Type)
				if typ == "" {
					errs = append(errs, errEntry{Index: i, Error: "type is required"})
					continue
				}
				target := assetTarget(item)
				if target != "" {
					ok, reason, _, scopeErr := store.IsInScope(ctx, taskID, target)
					if scopeErr != nil {
						errs = append(errs, errEntry{Index: i, Error: "scope check failed: " + scopeErr.Error()})
						continue
					}
					if !ok {
						errs = append(errs, errEntry{Index: i, Error: fmt.Sprintf("out of scope (%s): %s", RedactURLUserinfo(target), reason)})
						continue
					}
				}
				id, err := store.InsertAsset(ctx, taskID, item)
				if err != nil {
					errs = append(errs, errEntry{Index: i, Error: err.Error()})
					continue
				}
				results = append(results, result{Index: i, ID: id, Type: typ})
			}

			return jsonResult(map[string]any{
				"results": results,
				"errors":  errs,
			})
		},
	}
}

func makeListAssets(store Store, taskID int64) Tool {
	return Tool{
		Name:     "list_assets",
		Category: CategoryAssets,
		Description: "Query the asset library: search by DSL expression, or fetch directly by id/ids; supports pagination.\n" +
			"DSL: field=value fuzzy (ILIKE) | field==value exact | field!=value exclusion | numeric fields support > >= < <= | a bare word = full-text fuzzy; combine with AND/OR (AND binds tighter), parentheses for grouping.\n" +
			"When neither id nor ids is passed, dsl must be non-empty.\n" +
			"Available fields: domain, root_domain, ip, url, page_title, icp, service_name, app_name, method, service_type, record_type, technology, port, status_code, company_id.",
		ReadOnly: true,
		Schema: obj(map[string]any{
			"dsl":    str("DSL query expression (must be non-empty when id/ids are not passed)"),
			"type":   str("asset type filter: root_domain|ip|subdomain|app|service|endpoint"),
			"id":     intp("fetch by a single asset id"),
			"ids":    arr(map[string]any{"type": "integer"}, "fetch by multiple asset ids"),
			"limit":  intp("return cap, default 10"),
			"offset": intp("pagination offset, default 0"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				DSL    string  `json:"dsl"`
				Type   string  `json:"type"`
				ID     int64   `json:"id"`
				IDs    []int64 `json:"ids"`
				Limit  int     `json:"limit"`
				Offset int     `json:"offset"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if a.Limit <= 0 {
				a.Limit = 10
			}

			if a.ID <= 0 && len(a.IDs) == 0 && strings.TrimSpace(a.DSL) == "" {
				return errResult("dsl must not be empty when id/ids are not passed"), nil
			}

			assets, total, err := store.ListAssets(ctx, AssetFilter{
				TaskID: taskID,
				DSL:    a.DSL,
				Type:   a.Type,
				ID:     a.ID,
				IDs:    a.IDs,
				Limit:  a.Limit,
				Offset: a.Offset,
			})
			if err != nil {
				return errResult("query error: " + err.Error()), nil
			}
			return jsonResult(map[string]any{
				"count":  total,
				"assets": assets,
			})
		},
	}
}

func makeListCompanies(store Store) Tool {
	return Tool{
		Name:     "list_companies",
		Category: CategoryAssets,
		Description: "List the companies in the asset library with their asset scopes and attributed asset counts. " +
			"The optional search fuzzy-filters by company name (case-insensitive); empty returns all.",
		ReadOnly: true,
		Schema: obj(map[string]any{
			"search": str("fuzzy filter by company name (optional, case-insensitive)"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Search string `json:"search"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}

			cos, err := store.ListCompanies(ctx)
			if err != nil {
				return errResult(err.Error()), nil
			}

			q := strings.ToLower(strings.TrimSpace(a.Search))
			out := make([]Company, 0, len(cos))
			for _, c := range cos {
				if q != "" && !strings.Contains(strings.ToLower(c.Name), q) {
					continue
				}
				out = append(out, c)
			}
			return jsonResult(map[string]any{"count": len(out), "companies": out})
		},
	}
}

func makeAddCompanyScope(store Store) Tool {
	return Tool{
		Name:     "add_company_scope",
		Category: CategoryAssets,
		Description: "Add domains/IPs/CIDRs/ICP registrations/company keywords to a company's asset scope. " +
			"Company names are unique: if the company doesn't exist it is created; if it exists it is reused.\n" +
			"This alone does not change what is in scope. Once the company is added to a task with add_task_scope kind=company, its domain, IP and CIDR lines are enforced as part of that task's scope; ICP and keyword lines are recorded for attribution only.\n" +
			"scope is one entry per line; the system auto-recognizes: root domain / URL / single IP / CIDR range / ICP registration / company keyword.\n" +
			"Always give reason explaining the attribution basis.",
		Schema: obj(map[string]any{
			"company": str("company name (created if absent, reused if present)"),
			"scope":   str("asset scope, one entry per line"),
			"reason":  str("attribution basis (evidence/source); required"),
			"logo":    str("company logo URL (optional)"),
		}, "company", "scope"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Company string `json:"company"`
				Scope   string `json:"scope"`
				Reason  string `json:"reason"`
				Logo    string `json:"logo"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if strings.TrimSpace(a.Company) == "" {
				return errResult("company must not be empty"), nil
			}
			companyID, err := store.GetOrCreateCompany(ctx, a.Company, a.Logo)
			if err != nil {
				return errResult(err.Error()), nil
			}
			lines := splitLines(a.Scope)
			added, skipped, invalid, errMsgs, err := store.AddCompanyScope(ctx, companyID, lines, a.Reason)
			if err != nil {
				return errResult(err.Error()), nil
			}
			out := map[string]any{
				"company_id": companyID,
				"added":      added,
				"skipped":    skipped,
				"invalid":    invalid,
			}
			if len(errMsgs) > 0 {
				out["errors"] = errMsgs
			}
			return jsonResult(out)
		},
	}
}

func makeAddTaskScope(store Store, taskID int64) Tool {
	return Tool{
		Name:     "add_task_scope",
		Category: CategoryAssets,
		Description: "Add entries to this task's scope. Scope is enforced: insert_assets, the enrich tools, traffic_replay and capture_command_output refuse targets outside it, and check_scope tests a target against it. " +
			"Targets are matched against root_domain, subdomain, ip and cidr entries and, for each company entry, the domain, IP and CIDR lines add_company_scope recorded for that company. A task with none of those allows every target, so the first one restricts the task to what is listed and later ones widen it. " +
			"A company entry whose company has no domain, IP or CIDR lines allows nothing on its own rather than everything; add them with add_company_scope. " +
			"Task scope is also the denominator of list_untested_assets, which counts a company entry's assets too. icp and keyword entries are not matched or counted; plugins receive every entry plus each company's scope lines.\n" +
			"Supported kinds: company / root_domain / subdomain / ip / cidr / icp / keyword.\n" +
			"Use the entries array for multiple entries.",
		Schema: obj(map[string]any{
			"entries": map[string]any{"type": "array", "description": "batch: [{kind, value}]", "items": map[string]any{"type": "object"}},
			"kind":    str("company / root_domain / subdomain / ip / cidr / icp / keyword"),
			"value":   str("company name or id / domain / IP / CIDR / ICP / keyword"),
			"reason":  str("basis for adding (for audit); required"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			if taskID <= 0 {
				return errResult("add_task_scope requires a task context"), nil
			}
			type scopeEntry struct {
				Kind  string `json:"kind"`
				Value string `json:"value"`
			}
			var a struct {
				Entries []scopeEntry `json:"entries"`
				Kind    string       `json:"kind"`
				Value   string       `json:"value"`
				Reason  string       `json:"reason"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			items := a.Entries
			if len(items) > maxBatchLen {
				return errResult(fmt.Sprintf("entries array too large (%d items, max %d)", len(items), maxBatchLen)), nil
			}
			if len(items) == 0 {
				items = []scopeEntry{{Kind: a.Kind, Value: a.Value}}
			}
			var added []map[string]any
			errs := map[string]string{}
			for i, e := range items {
				err := store.AddTaskScope(ctx, taskID, strings.TrimSpace(e.Kind), e.Value, a.Reason, "agent")
				if err != nil {
					errs[fmt.Sprintf("%d", i)] = err.Error()
					continue
				}
				added = append(added, map[string]any{"kind": e.Kind, "value": e.Value})
			}
			out := map[string]any{"added": added}
			if len(errs) > 0 {
				out["errors"] = errs
			}
			return jsonResult(out)
		},
	}
}

func makeListUntestedAssets(store Store, taskID int64) Tool {
	return Tool{
		Name:     "list_untested_assets",
		Category: CategoryAssets,
		Description: "Query the assets inside the scope of this task that haven't been covered by fact anchors yet.\n" +
			"Optional filter by asset type: root_domain/subdomain/service/app/endpoint/ip.\n" +
			"Pagination: page starts at 1, page_size defaults to 10. Returns {assets, total, page, page_size}.",
		ReadOnly: true,
		Schema: obj(map[string]any{
			"type":      str("asset type filter (optional)"),
			"page":      intp("page number, starting at 1 (default 1)"),
			"page_size": intp("entries per page (default 10)"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			if taskID <= 0 {
				return errResult("list_untested_assets requires a task context"), nil
			}
			var a struct {
				Type     string `json:"type"`
				Page     int    `json:"page"`
				PageSize int    `json:"page_size"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if a.Page <= 0 {
				a.Page = 1
			}
			if a.PageSize <= 0 {
				a.PageSize = 10
			}
			offset := (a.Page - 1) * a.PageSize
			assets, total, err := store.ListUntestedAssets(ctx, taskID, strings.TrimSpace(a.Type), a.PageSize, offset)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"assets": assets, "total": total, "page": a.Page, "page_size": a.PageSize,
			})
		},
	}
}

func assetTarget(item AssetInput) string {
	switch item.Type {
	case "root_domain", "subdomain":
		return item.Domain
	case "ip":
		return item.IP
	case "service", "endpoint":
		if item.URL != "" {
			return item.URL
		}
		if item.IP != "" {
			return item.IP
		}
		return item.Domain
	}
	return ""
}
