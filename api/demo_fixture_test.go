package api

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"reflect"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

var updateDemoFixture = flag.Bool("update-demo-fixture", false, "rewrite the /api/tools entry of the web demo fixture")

const demoRoutesPath = "../web/src/demo/fixtures/routes.json"

// TestDemoToolsFixture keeps the live demo's /api/tools response in step with
// the tools a `serve --web --proxy` instance lists.
func TestDemoToolsFixture(t *testing.T) {
	reg := tools.NewRegistry()
	tools.RegisterAll(reg, nil, 0, 0)
	tools.RegisterEvidenceTools(reg, nil, nil, 0)
	tools.RegisterTrafficTools(reg, nil, nil, 0)
	a := New(&mockStore{}, nil, 1, 1, WithToolList(reg.Tools()))

	var want bytes.Buffer
	enc := json.NewEncoder(&want)
	enc.SetEscapeHTML(false)
	enc.SetIndent(" ", " ")
	if err := enc.Encode(a.toolList); err != nil {
		t.Fatal(err)
	}

	file, err := os.ReadFile(demoRoutesPath)
	if err != nil {
		t.Fatal(err)
	}
	start, end := routeValueSpan(t, file, "/api/tools")

	if *updateDemoFixture {
		out := append(append(append([]byte{}, file[:start]...), bytes.TrimRight(want.Bytes(), "\n")...), file[end:]...)
		if err := os.WriteFile(demoRoutesPath, out, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	var got, exp any
	if err := json.Unmarshal(file[start:end], &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want.Bytes(), &exp); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, exp) {
		t.Fatalf("%s /api/tools differs from the tool catalog; run: go test ./api -run TestDemoToolsFixture -update-demo-fixture", demoRoutesPath)
	}
}

// routeValueSpan returns the byte range of one top-level value in the routes
// fixture, so it can be replaced without reformatting the other routes.
func routeValueSpan(t *testing.T, file []byte, route string) (int, int) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(file))
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			t.Fatal(err)
		}
		if key == route {
			end := int(dec.InputOffset())
			return end - len(raw), end
		}
	}
	t.Fatalf("%s has no %q entry", demoRoutesPath, route)
	return 0, 0
}
