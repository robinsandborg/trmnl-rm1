package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	for _, tc := range []struct {
		args      []string
		wantError bool
	}{
		{nil, false},
		{[]string{"-h"}, false},
		{[]string{"-since", "invalid"}, true},
		{[]string{"-since", "2026-09-14T00:00:00Z", "-until", "2026-09-13T00:00:00Z"}, true},
		{[]string{"unexpected"}, true},
	} {
		var out, errs bytes.Buffer
		if err := run(tc.args, strings.NewReader(""), &out, &errs); (err != nil) != tc.wantError {
			t.Fatalf("args %v: %v", tc.args, err)
		}
		if tc.args == nil && !strings.Contains(out.String(), `"schema_version": 1`) {
			t.Fatalf("missing report: %s", out.String())
		}
	}
}
