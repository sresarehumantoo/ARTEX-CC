---
title: Assets and scope
description: "Insert and query assets, manage company and task scope, and record coverage."
sidebar:
  order: 2
---

## `insert_assets`

Register newly discovered assets in bulk; multiple types can be mixed in one call.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| assets | array of objects | yes | Asset array; each element is one asset record (max 500) |
| assets[].type | string | yes | `root_domain`, `ip`, `subdomain`, `app`, `service`, or `endpoint` |
| assets[].domain | string | varies | Root domain or subdomain (required for root_domain/subdomain) |
| assets[].icp | string | no | ICP registration number |
| assets[].record_type | string | no | DNS record type: A/AAAA/CNAME/MX etc. (for subdomain) |
| assets[].record_value | array of string | no | DNS record values (for subdomain) |
| assets[].ip | string | varies | IPv4/IPv6 address (required for ip type, must not be a hostname) |
| assets[].bound_domains | array of string | no | Domains bound to this IP |
| assets[].open_ports | array of objects | no | Open ports, each with `port` (required) and `service` (optional) |
| assets[].app_name | string | varies | Application name (required for app type) |
| assets[].bundle_id | string | no | Bundle ID (for app) |
| assets[].category | string | no | Application category |
| assets[].description | string | no | Application description |
| assets[].app_icp | string | no | Application ICP registration |
| assets[].company_id | integer | no | Owning company ID |
| assets[].url | string | varies | Full URL including scheme and port (required for HTTP service and endpoint) |
| assets[].status_code | integer | no | HTTP response status code |
| assets[].content_length | integer | no | HTTP response body size in bytes |
| assets[].page_title | string | no | Page `<title>` content |
| assets[].favicon_mmh3 | string | no | Favicon MMH3 hash |
| assets[].technologies | array of string | no | Tech-stack list (append-merged, not overwritten) |
| assets[].auth | array of objects | no | Discovered credentials (append-merged, not overwritten) |
| assets[].service_name | string | varies | Service name, e.g. ssh/mysql/redis (required for non-HTTP service) |
| assets[].service_ip | string | no | Enrichment IP |
| assets[].port | integer | varies | Port number (required for non-HTTP service) |
| assets[].method | string | varies | HTTP method (required for endpoint) |
| assets[].params | array of objects | no | Request parameters (append-merged) |

**Returns:** `{results: [{index, id, type}, ...], errors: [{index, error}, ...]}`.

## `list_assets`

Query the asset library: search by DSL expression, or fetch directly by id/ids; supports pagination.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| dsl | string | varies | DSL query expression (required when id/ids are not passed) |
| type | string | no | Asset type filter: `root_domain`, `ip`, `subdomain`, `app`, `service`, `endpoint` |
| id | integer | no | Fetch by a single asset ID |
| ids | array of integer | no | Fetch by multiple asset IDs |
| limit | integer | no | Return cap, default 10 |
| offset | integer | no | Pagination offset, default 0 |

DSL syntax: `field=value` (fuzzy/ILIKE), `field==value` (exact), `field!=value` (exclusion), numeric fields support `>`, `>=`, `<`, `<=`, bare word is full-text fuzzy. Combine with AND/OR (AND binds tighter), parentheses for grouping. Available fields: `domain`, `root_domain`, `ip`, `url`, `page_title`, `icp`, `service_name`, `app_name`, `method`, `service_type`, `record_type`, `technology`, `port`, `status_code`, `company_id`.

**Returns:** `{count, assets: [...]}`.

## `list_companies`

List companies in the asset library with their asset scopes and attributed asset counts.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| search | string | no | Fuzzy filter by company name (case-insensitive) |

**Returns:** `{count, companies: [...]}`.

## `add_company_scope`

Add domains/IPs/CIDRs/ICP registrations/company keywords to a company's asset scope. Company names are unique: if the company doesn't exist it is created.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| company | string | yes | Company name (created if absent, reused if present) |
| scope | string | yes | Asset scope, one entry per line (auto-recognized: root domain / URL / single IP / CIDR range / ICP / company keyword) |
| reason | string | no | Attribution basis (evidence/source) |
| logo | string | no | Company logo URL |

**Returns:** `{company_id, added, skipped, invalid, errors?}`.

## `add_task_scope`

Add test scope to this task (coverage scope, not enforced): it is the denominator of asset test coverage.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| entries | array of objects | no | Batch: `[{kind, value}, ...]` |
| kind | string | no | `company`, `root_domain`, `subdomain`, `ip`, `cidr`, `icp`, or `keyword` |
| value | string | no | Company name or ID / domain / IP / CIDR / ICP / keyword |
| reason | string | no | Basis for adding (for audit) |

Use `entries` for batch or `kind`+`value` for a single entry.

**Returns:** `{added: [{kind, value}, ...], errors?}`.

## `list_untested_assets`

Query assets inside the scope of this task that haven't been covered by fact anchors yet.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| type | string | no | Asset type filter: `root_domain`, `subdomain`, `service`, `app`, `endpoint`, `ip` |
| page | integer | no | Page number, starting at 1 (default 1) |
| page_size | integer | no | Entries per page (default 10) |

**Returns:** `{assets: [...], total, page, page_size}`.

## `check_scope`

Check whether a target (domain, IP, or URL) is within the engagement's scope rules.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| target | string | yes | Domain, IP address, or URL to check |

**Returns:** `{in_scope: bool, reason, matching_rule}`.

## `record_coverage`

Record what you examined on a surface for one risk class, including negative results, so an absent finding is never read as a clean target.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| surface | string | yes | What was examined: a URL, endpoint, parameter, host, or feature |
| risk_area | string | yes | Risk class, e.g. `sqli`, `xss`, `idor` |
| outcome | string | yes | `reported`, `no_issue_found`, `ruled_out`, `not_applicable`, or `needs_follow_up` |
| evidence | string | no | Required for `no_issue_found`: a fact ID, intent ID, or traffic ID that shows the test |
| reason | string | no | Required for `ruled_out` and `not_applicable`: why |

**Returns:** `"recorded"`.
