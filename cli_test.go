package cliby

import (
	"encoding/json"
	"github.com/op/go-logging"
	"github.com/pmezard/go-difflib/difflib"
	"os"
	"reflect"
	"testing"
)

var TestOptionMergeExpected = map[string]interface{}{
	"config-file": ".test.d/config.yml",
	"a":           1,
	"b":           999,
	"A":           1,
	"hash": map[string]interface{}{
		"a": 1,
		"b": 999,
		"A": 1,
		"hoh": map[string]interface{}{
			"a": 1,
			"b": 999,
			"A": 1,
		},
		"hol": map[string]interface{}{
			"a": []interface{}{2, 999, 1},
			"b": []interface{}{998, 999, 3, 4},
		},
	},
	"list": []interface{}{
		"b",
		"A",
		"B",
		"a",
	},
	"lol": []interface{}{
		[]interface{}{"A", "B", "C"},
		[]interface{}{"d", "e", "f"},
		[]interface{}{"a", "b", "c"},
	},
}

func TestOptionMerge(t *testing.T) {
	InitLogging()
	logging.SetLevel(logging.DEBUG, "")
	cli := New("test")
	cli.Opts = map[string]interface{}{
		"a": 1,
		"b": 2,
		"hash": map[string]interface{}{
			"a": 1,
			"b": 2,
			"hoh": map[string]interface{}{
				"a": 1,
				"b": 2,
			},
			"hol": map[string]interface{}{
				"a": []interface{}{1, 2},
				"b": []interface{}{3, 4},
			},
		},
		"list": []interface{}{
			"a",
			"b",
		},
		"lol": []interface{}{
			[]interface{}{"a", "b", "c"},
			[]interface{}{"d", "e", "f"},
		},
	}

	cli.ProcessOptions()
	if !reflect.DeepEqual(cli.Opts, TestOptionMergeExpected) {
		got, _ := json.MarshalIndent(cli.Opts, "", "    ")
		expected, _ := json.MarshalIndent(TestOptionMergeExpected, "", "    ")

		diff := difflib.UnifiedDiff{
			A:        difflib.SplitLines(string(expected)),
			B:        difflib.SplitLines(string(got)),
			FromFile: "Expected",
			ToFile:   "Got",
			Context:  3,
		}
		result, _ := difflib.GetUnifiedDiffString(diff)
		log.Error("Diff:\n%s", result)
		t.Fail()
	}
}

var TestOptionMergeSubdirExpected = map[string]interface{}{
	"config-file": ".test.d/config.yml",
	"a":           1,
	"b":           101,
	"A":           1,
	"C":           1,
	"hash": map[string]interface{}{
		"a": 1,
		"b": 101,
		"A": 1,
		"C": 1,
		"hoh": map[string]interface{}{
			"a": 1,
			"b": 101,
			"A": 1,
			"C": 1,
		},
		"hol": map[string]interface{}{
			"a": []interface{}{2, 101, 999, 1},
			"b": []interface{}{101, 102, 998, 999, 3, 4},
		},
	},
	"list": []interface{}{
		"b",
		"C",
		"A",
		"B",
		"a",
	},
	"lol": []interface{}{
		[]interface{}{"D", "E", "F"},
		[]interface{}{"d", "e", "f"},
		[]interface{}{"A", "B", "C"},
		[]interface{}{"a", "b", "c"},
	},
}

func TestOptionMergeSubdir(t *testing.T) {
	os.Chdir("subdir")
	InitLogging()
	logging.SetLevel(logging.DEBUG, "")
	cli := New("test")
	cli.Opts = map[string]interface{}{
		"a": 1,
		"b": 2,
		"hash": map[string]interface{}{
			"a": 1,
			"b": 2,
			"hoh": map[string]interface{}{
				"a": 1,
				"b": 2,
			},
			"hol": map[string]interface{}{
				"a": []interface{}{1, 2},
				"b": []interface{}{3, 4},
			},
		},
		"list": []interface{}{
			"a",
			"b",
		},
		"lol": []interface{}{
			[]interface{}{"a", "b", "c"},
			[]interface{}{"d", "e", "f"},
		},
	}

	cli.ProcessOptions()
	if !reflect.DeepEqual(cli.Opts, TestOptionMergeSubdirExpected) {
		got, _ := json.MarshalIndent(cli.Opts, "", "    ")
		expected, _ := json.MarshalIndent(TestOptionMergeSubdirExpected, "", "    ")

		diff := difflib.UnifiedDiff{
			A:        difflib.SplitLines(string(expected)),
			B:        difflib.SplitLines(string(got)),
			FromFile: "Expected",
			ToFile:   "Got",
			Context:  3,
		}
		result, _ := difflib.GetUnifiedDiffString(diff)
		log.Error("Diff:\n%s", result)
		t.Fail()
	}
}
