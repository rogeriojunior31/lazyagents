package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"testing"
)

func TestDoctorSharesActionableReportsAndPreservesJSON(t *testing.T) {
	externalCalls := 0
	checks := []Check{
		{Title: "local", NextStep: "Repair the link and retry.", Tab: "skills", Run: func(c Context, out io.Writer) []Problem {
			_, _ = io.WriteString(out, "broken link\n")
			return []Problem{{Resource: "skill-link", Explanation: "broken link"}}
		}},
		{Title: "remote", External: true, NextStep: "Check connectivity and retry explicitly.", Tab: "usage", Run: func(c Context, out io.Writer) []Problem {
			externalCalls++
			return []Problem{{Resource: "fixture", Explanation: "HTTP 500"}}
		}},
	}
	local := RunDoctor(context.Background(), Context{}, checks, false)
	if externalCalls != 0 || local.OK || local.Scope != "local" || !local.Sections[1].Skipped {
		t.Fatalf("wrong local scope: %+v", local)
	}
	p := local.Sections[0].Findings[0]
	if p.Severity != "warning" || p.Resource != "skill-link" || p.NextStep != checks[0].NextStep || p.Tab != "skills" {
		t.Fatalf("missing action: %+v", p)
	}
	var out, errOut bytes.Buffer
	c := Context{Out: &out, Err: &errOut}
	if code := DoctorCommand(checks).Run(c, []string{"--json", "--local"}); code != 1 {
		t.Fatal(code)
	}
	var decoded DoctorReport
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(local, decoded) || len(decoded.Sections[0].Problems) != 1 || decoded.Sections[0].Problems[0] != "broken link" || decoded.Sections[0].Report != "broken link" {
		t.Fatalf("JSON/report mismatch: %s", &out)
	}
	out.Reset()
	if code := DoctorCommand(checks).Run(c, []string{"--json"}); code != 1 {
		t.Fatal(code)
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if externalCalls != 1 || decoded.Scope != "all" || decoded.Sections[1].Skipped || decoded.Sections[1].Findings[0].Explanation != "HTTP 500" {
		t.Fatalf("external check did not run explicitly: %+v", decoded)
	}
}

func TestDoctorCancellationAndInvalidArgumentsDoNotRunMoreChecks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checks := []Check{
		{Title: "cancel", Run: func(c Context, out io.Writer) []Problem { cancel(); return nil }},
		{Title: "next", Run: func(c Context, out io.Writer) []Problem { t.Fatal("check started after cancellation"); return nil }},
	}
	report := RunDoctor(ctx, Context{}, checks, true)
	if !report.Cancelled || report.OK || !report.Sections[1].Skipped {
		t.Fatalf("cancellation lost: %+v", report)
	}
	checks[0].Run = func(c Context, out io.Writer) []Problem { t.Fatal("invalid arguments ran checks"); return nil }
	var out bytes.Buffer
	if code := DoctorCommand(checks).Run(Context{Out: &out, Err: &out}, []string{"extra", "--local"}); code != 1 {
		t.Fatal(code)
	}
}
