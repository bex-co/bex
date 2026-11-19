/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package objectstorage_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

func TestParseLifecycleReadsTheStoredShapeAndToleratesUnknownFields(t *testing.T) {
	raw := json.RawMessage(`[
		{"id":"expire-logs","enabled":true,"conditions":{"prefix":"logs/"},
		 "deleteObjectsTransition":{"condition":{"type":"Age","maxAge":86400}}},
		{"id":"until","enabled":false,"conditions":{"prefix":""},
		 "deleteObjectsTransition":{"condition":{"type":"Date","date":"2030-01-01T00:00:00.000Z","somethingNew":1}},
		 "futureField":{"a":[1,2]}},
		{"id":"ia","enabled":true,"conditions":{"prefix":"cold/"},
		 "storageClassTransitions":[{"condition":{"type":"Age","maxAge":60},"storageClass":"InfrequentAccess"}],
		 "abortMultipartUploadsTransition":{"condition":{"type":"Age","maxAge":60}}}
	]`)
	rules, err := objectstorage.ParseLifecycle(raw)
	if err != nil || len(rules) != 3 {
		t.Fatalf("ParseLifecycle = %+v, %v; want 3 rules", rules, err)
	}
	age := rules[0]
	if age.ID != "expire-logs" || !age.Enabled || age.Conditions.Prefix != "logs/" || age.DeleteObjectsTransition == nil ||
		age.DeleteObjectsTransition.Condition.Type != objectstorage.LifecycleAge || age.DeleteObjectsTransition.Condition.MaxAge == nil || *age.DeleteObjectsTransition.Condition.MaxAge != 86400 {
		t.Fatalf("age rule = %+v", age)
	}
	date := rules[1]
	if date.Enabled || date.DeleteObjectsTransition.Condition.Type != objectstorage.LifecycleDate || date.DeleteObjectsTransition.Condition.Date != "2030-01-01T00:00:00.000Z" {
		t.Fatalf("date rule = %+v", date)
	}
	// Transitions that do nothing are kept verbatim, so a rule reads back as written.
	inert := rules[2]
	if inert.DeleteObjectsTransition != nil || len(inert.StorageClassTransitions) == 0 || len(inert.AbortMultipartUploadsTransition) == 0 {
		t.Fatalf("inert rule = %+v", inert)
	}
}

func TestParseLifecycleOfNothingIsNoRules(t *testing.T) {
	for _, raw := range []string{"", "  ", "null", "[]"} {
		rules, err := objectstorage.ParseLifecycle(json.RawMessage(raw))
		if err != nil || len(rules) != 0 {
			t.Errorf("ParseLifecycle(%q) = %+v, %v; want no rules and no error", raw, rules, err)
		}
	}
}

func TestParseLifecycleRefusesWhatIsNotARuleList(t *testing.T) {
	for name, raw := range map[string]string{
		"an object":       `{"rules":[]}`,
		"a string":        `"x"`,
		"a number":        `7`,
		"truncated":       `[{"id":`,
		"a string rule":   `["x"]`,
		"a string maxAge": `[{"id":"a","enabled":true,"deleteObjectsTransition":{"condition":{"type":"Age","maxAge":"soon"}}}]`,
		"enabled as text": `[{"id":"a","enabled":"yes"}]`,
	} {
		if _, err := objectstorage.ParseLifecycle(json.RawMessage(raw)); !errors.Is(err, objectstorage.ErrInvalidLifecycle) {
			t.Errorf("%s: ParseLifecycle(%s) = %v, want ErrInvalidLifecycle", name, raw, err)
		}
	}
}

func TestLifecycleRuleValidate(t *testing.T) {
	rule := func(condition string) objectstorage.LifecycleRule {
		t.Helper()
		var r objectstorage.LifecycleRule
		raw := `{"id":"r","enabled":true,"conditions":{"prefix":""},"deleteObjectsTransition":{"condition":` + condition + `}}`
		if err := json.Unmarshal([]byte(raw), &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	for name, c := range map[string]string{
		"age":                 `{"type":"Age","maxAge":1}`,
		"date and time":       `{"type":"Date","date":"2030-01-01T00:00:00Z"}`,
		"date with fraction":  `{"type":"Date","date":"2030-01-01T00:00:00.000Z"}`,
		"a bare date":         `{"type":"Date","date":"2030-01-01"}`,
		"a date with offset":  `{"type":"Date","date":"2030-01-01T09:00:00+09:00"}`,
		"a date with spaces":  `{"type":"Date","date":" 2030-01-01 "}`,
		"age with extra date": `{"type":"Age","maxAge":5,"date":"junk"}`,
		"age of zero":         `{"type":"Age","maxAge":0}`,
		"the longest age":     fmt.Sprintf(`{"type":"Age","maxAge":%d}`, objectstorage.MaxLifecycleAgeSeconds),
	} {
		if err := rule(c).Validate(); err != nil {
			t.Errorf("%s: Validate = %v, want valid", name, err)
		}
	}
	for name, c := range map[string]string{
		"age without maxAge": `{"type":"Age"}`,
		"age as null":        `{"type":"Age","maxAge":null}`,
		"negative age":       `{"type":"Age","maxAge":-5}`,
		// A "never expire" idiom: as a time.Duration of nanoseconds it wraps
		// negative, and every object would look expired.
		"age past 100 years": `{"type":"Age","maxAge":10000000000}`,
		"date without date":  `{"type":"Date"}`,
		"date that is junk":  `{"type":"Date","date":"next tuesday"}`,
		"no type":            `{"maxAge":5}`,
		"another type":       `{"type":"Weekly","maxAge":5}`,
		"lower case type":    `{"type":"age","maxAge":5}`,
	} {
		if err := rule(c).Validate(); !errors.Is(err, objectstorage.ErrInvalidLifecycle) {
			t.Errorf("%s: Validate = %v, want ErrInvalidLifecycle", name, err)
		}
	}
	// A rule that only transitions storage class or aborts multipart uploads is
	// valid and does nothing.
	if err := (objectstorage.LifecycleRule{ID: "ia", Enabled: true}).Validate(); err != nil {
		t.Errorf("a rule with no delete transition = %v, want valid", err)
	}
}
