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

func TestReportWriteText_RealRunShowsPushedAndFailures(t *testing.T) {
	rep := &Report{Pushed: 2, Failed: 1, ExistingLinks: 4, Imported: 5}
	var buf bytes.Buffer
	if err := rep.WriteText(&buf); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := strings.ToLower(buf.String())
	for _, want := range []string{"2", "push", "1", "fail", "4", "link", "5", "import"} {
		if !strings.Contains(out, want) {
			t.Errorf("text report missing %q:\n%s", want, buf.String())
		}
	}
	if strings.Contains(out, "dry") {
		t.Errorf("real run should not say dry:\n%s", buf.String())
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
