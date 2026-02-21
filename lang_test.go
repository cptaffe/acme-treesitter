package treesitter

import "testing"

// TestQueryCompilation checks that every registered language has a query that
// compiled successfully against its grammar.  A failed query is logged (not
// fatal) at startup and results in a Language with a nil query field; this
// test makes that failure visible at CI time.
func TestQueryCompilation(t *testing.T) {
	// initLanguages is called from init(); by the time the test runs all
	// entries in langByName have been populated.
	for name, l := range langByName {
		if l.query == nil {
			t.Errorf("%s: query failed to compile (check log above for offset/message)", name)
		} else {
			t.Logf("%s: ok (%d patterns)", name, l.query.PatternCount())
		}
	}
}
