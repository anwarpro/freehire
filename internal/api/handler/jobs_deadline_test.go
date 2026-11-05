package handler

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDeadlinePatchPresence(t *testing.T) {
	for _, tc := range []struct {
		body                  string
		present, valid, clear bool
	}{
		{`{}`, false, true, true},
		{`{"application_deadline":null}`, true, true, true},
		{`{"application_deadline":"2026-12-31T18:00:00Z"}`, true, true, false},
		{`{"application_deadline":"bad"}`, true, false, true},
		{`{"application_deadline":123}`, true, false, true},
	} {
		t.Run(tc.body, func(t *testing.T) {
			var in updateJobRequest
			if err := json.Unmarshal([]byte(tc.body), &in); err != nil {
				t.Fatal(err)
			}
			if (len(in.ApplicationDeadline) > 0) != tc.present {
				t.Fatal("lost presence")
			}
			var deadline *time.Time
			var err error
			if len(in.ApplicationDeadline) > 0 {
				err = json.Unmarshal(in.ApplicationDeadline, &deadline)
			}
			if (err == nil) != tc.valid {
				t.Fatalf("decode error: %v", err)
			}
			if tc.valid && (deadline == nil) != tc.clear {
				t.Fatal("lost null")
			}
		})
	}
}
