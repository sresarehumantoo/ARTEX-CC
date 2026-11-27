package traffic

import (
	"strings"
	"testing"
)

func queryPlan(t *testing.T, tr *Traffic, q string, args ...any) string {
	t.Helper()
	rows, err := tr.db.Query("EXPLAIN QUERY PLAN "+q, args...)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		plan = append(plan, detail)
	}
	return strings.Join(plan, "\n")
}

func TestHostQueriesUseHostTSIndex(t *testing.T) {
	tr := openTestTraffic(t)

	plan := queryPlan(t, tr, `SELECT id FROM exchanges WHERE host=? ORDER BY ts DESC LIMIT 3`, "a.example")
	if !strings.Contains(plan, "idx_ex_host_ts") || strings.Contains(plan, "TEMP B-TREE") {
		t.Fatalf("host lookup should walk idx_ex_host_ts without a sort:\n%s", plan)
	}
	plan = queryPlan(t, tr, `SELECT host, COUNT(*) AS n, MAX(ts) AS last FROM exchanges GROUP BY host ORDER BY last DESC`)
	if !strings.Contains(plan, "COVERING INDEX idx_ex_host_ts") {
		t.Fatalf("Hosts() should read only idx_ex_host_ts:\n%s", plan)
	}
}
