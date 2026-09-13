package sync

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestReportWriteText_ShowsCountsAndDryRun(t *testing.T) {
	rep := &Report{DryRun: true, WouldPush: 2, WouldImport: 3, ExistingLinks: 4}
	var buf bytes.Buffer
	if err := rep.WriteText(&buf); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"2", "3", "4", "push", "import"} {
		if !strings.Contains(strings.ToLower(out), strings.ToLower(want)) {
			t.Errorf("text report missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(strings.ToLower(out), "dry") {
		t.Errorf("dry-run report should say so:\n%s", out)
	}
}

func TestReportWriteJSON_RoundTrips(t *testing.T) {
	rep := &Report{DryRun: true, WouldPush: 2, WouldImport: 3, ExistingLinks: 4}
	var buf bytes.Buffer
	if err := rep.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var got Report
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if got != *rep {
		t.Errorf("round-trip = %+v, want %+v", got, *rep)
	}
}
