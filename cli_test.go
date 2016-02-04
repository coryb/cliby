package cliby

import (
	"encoding/json"
	"github.com/op/go-logging"
	"github.com/pmezard/go-difflib/difflib"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"reflect"
	"testing"
)

var TestOptionMergeExpected = map[string]interface{}{
	"a": 1,
	"b": 999,
	"A": 1,
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

type TestCli struct {
	Cli
}

func (c *TestCli) CommandLine() *kingpin.Application {
	return kingpin.New("test", "test app")
}

func (c *TestCli) NewOptions() interface{} {
	return map[string]interface{}{}
}

func TestOptionMerge(t *testing.T) {
	InitLogging()
	logging.SetLevel(logging.DEBUG, "")
	cli := &TestCli{*New("test")}

	cli.SetDefaults(map[string]interface{}{
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
	})

	os.Args = []string{os.Args[0]}

	ProcessAllOptions(cli)
	log.Debug("processed: %#v", cli.GetOptions())
	options := cli.GetOptions()
	if !reflect.DeepEqual(options, TestOptionMergeExpected) {
		log.Debug("processed: %#v", options)
		got, err := json.MarshalIndent(options, "", "    ")
		if err != nil {
			log.Error("Failed to marshal json: %s", err)
		}
		log.Debug("got: %#v", string(got))
		log.Debug("processed: %#v", options)
		got, err = json.MarshalIndent(options, "", "    ")
		log.Debug("got: %#v", string(got))
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
	"a": 1,
	"b": 101,
	"A": 1,
	"C": 1,
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
	cli := &TestCli{*New("test")}
	cli.SetDefaults(map[string]interface{}{
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
	})

	ProcessAllOptions(cli)
	if !reflect.DeepEqual(cli.GetOptions(), TestOptionMergeSubdirExpected) {
		got, _ := json.MarshalIndent(cli.GetOptions(), "", "    ")
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
