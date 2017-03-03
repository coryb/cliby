package util

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/bmatcuk/doublestar"
	"github.com/mgutz/ansi"
	"github.com/op/go-logging"
	"gopkg.in/coryb/yaml.v2"
)

var log = logging.MustGetLogger("util")

func FindParentPaths(fileName string) []string {
	cwd, _ := os.Getwd()

	paths := make([]string, 0)

	// special case if homedir is not in current path then check there anyway
	homedir := os.Getenv("HOME")
	if !strings.HasPrefix(cwd, homedir) {
		file := fmt.Sprintf("%s/%s", homedir, fileName)
		if _, err := os.Stat(file); err == nil {
			paths = append(paths, file)
		}
	}

	var dir string
	for _, part := range strings.Split(cwd, string(os.PathSeparator)) {
		if dir == "/" {
			dir = fmt.Sprintf("/%s", part)
		} else {
			dir = fmt.Sprintf("%s/%s", dir, part)
		}
		file := fmt.Sprintf("%s/%s", dir, fileName)
		if _, err := os.Stat(file); err == nil {
			paths = append(paths, file)
		}
	}
	return paths
}

func FindClosestParentPath(fileName string) (string, error) {
	paths := FindParentPaths(fileName)
	if len(paths) > 0 {
		return paths[len(paths)-1], nil
	}
	return "", errors.New(fmt.Sprintf("%s not found in parent directory hierarchy", fileName))
}

func ReadFile(file string) string {
	var bytes []byte
	var err error
	if bytes, err = ioutil.ReadFile(file); err != nil {
		log.Error("Failed to read file %s: %s", file, err)
		os.Exit(1)
	}
	return string(bytes)
}

func CopyFile(src, dst string) (err error) {
	var s, d *os.File
	if s, err = os.Open(src); err == nil {
		defer s.Close()
		if d, err = os.Create(dst); err == nil {
			if _, err = io.Copy(d, s); err != nil {
				d.Close()
				return
			}
			return d.Close()
		}
	}
	return
}

func FuzzyAge(start string) (string, error) {
	if t, err := time.Parse("2006-01-02T15:04:05.000-0700", start); err != nil {
		return "", err
	} else {
		delta := time.Now().Sub(t)
		if delta.Minutes() < 2 {
			return "a minute", nil
		} else if dm := delta.Minutes(); dm < 45 {
			return fmt.Sprintf("%d minutes", int(dm)), nil
		} else if dm := delta.Minutes(); dm < 90 {
			return "an hour", nil
		} else if dh := delta.Hours(); dh < 24 {
			return fmt.Sprintf("%d hours", int(dh)), nil
		} else if dh := delta.Hours(); dh < 48 {
			return "a day", nil
		} else {
			return fmt.Sprintf("%d days", int(delta.Hours()/24)), nil
		}
	}
	return "unknown", nil
}

func RunTemplate(templateContent string, data interface{}, out io.Writer) error {

	if out == nil {
		out = os.Stdout
	}

	funcs := map[string]interface{}{
		"toJson": func(content interface{}) (string, error) {
			if bytes, err := json.MarshalIndent(content, "", "    "); err != nil {
				return "", err
			} else {
				return string(bytes), nil
			}
		},
		"toYAML": func(content interface{}) (string, error) {
			if bytes, err := yaml.Marshal(content); err != nil {
				return "", err
			} else {
				return string(bytes), nil
			}
		},
		"append": func(more string, content interface{}) (string, error) {
			switch value := content.(type) {
			case string:
				return string(append([]byte(content.(string)), []byte(more)...)), nil
			case []byte:
				return string(append(content.([]byte), []byte(more)...)), nil
			default:
				return "", errors.New(fmt.Sprintf("Unknown type: %s", value))
			}
		},
		"indent": func(spaces int, content string) string {
			indent := make([]rune, spaces+1, spaces+1)
			indent[0] = '\n'
			for i := 1; i < spaces+1; i += 1 {
				indent[i] = ' '
			}

			lineSeps := []rune{'\n', '\u0085', '\u2028', '\u2029'}
			for _, sep := range lineSeps {
				indent[0] = sep
				content = strings.Replace(content, string(sep), string(indent), -1)
			}
			return content

		},
		"comment": func(content string) string {
			lineSeps := []rune{'\n', '\u0085', '\u2028', '\u2029'}
			for _, sep := range lineSeps {
				content = strings.Replace(content, string(sep), string([]rune{sep, '#', ' '}), -1)
			}
			return content
		},
		"color": func(color string) string {
			return ansi.ColorCode(color)
		},
		"split": func(sep string, content string) []string {
			return strings.Split(content, sep)
		},
		"abbrev": func(max int, content string) string {
			if len(content) > max {
				var buffer bytes.Buffer
				buffer.WriteString(content[:max-3])
				buffer.WriteString("...")
				return buffer.String()
			}
			return content
		},
		"rep": func(count int, content string) string {
			var buffer bytes.Buffer
			for i := 0; i < count; i += 1 {
				buffer.WriteString(content)
			}
			return buffer.String()
		},
		"age": func(content string) (string, error) {
			return FuzzyAge(content)
		},
		"toUpper": func(content string) string {
			return strings.ToUpper(content)
		},
		"toLower": func(content string) string {
			return strings.ToLower(content)
		},
		"ftime": func(format string, t int64) string {
			return time.Unix(t, 0).Format(format)
		},
		"env": func(name string) string {
			return os.Getenv(name)
		},
		"hasPrefix": func(content, prefix string) bool {
			return strings.HasPrefix(content, prefix)
		},
		"hasSuffix": func(content, suffix string) bool {
			return strings.HasSuffix(content, suffix)
		},
		"trimPrefix": func(content, prefix string) string {
			return strings.TrimPrefix(content, prefix)
		},
		"trimSuffix": func(content, suffix string) string {
			return strings.TrimSuffix(content, suffix)
		},
		"cwd": func() (string, error) {
			return os.Getwd()
		},
		"findLatestArtifact": func(content string) (string, error) {
			matches, err := doublestar.Glob(content)
			if err != nil {
				return "", err
			}
			artifact := struct {
				Path string
				Info os.FileInfo
			}{}
			for _, matchPath := range matches {
				info, err := os.Stat(matchPath)
				if err != nil {
					return "", err
				}
				if artifact.Path == "" {
					artifact.Info = info
					artifact.Path = matchPath
				} else if artifact.Info.ModTime().Before(info.ModTime()) {
					artifact.Info = info
					artifact.Path = matchPath
				}
			}
			if artifact.Path == "" {
				return "", fmt.Errorf("No Artifact Found matching glob: %v", content)
			}
			return artifact.Path, nil
		},
	}
	if tmpl, err := template.New("template").Funcs(funcs).Parse(templateContent); err != nil {
		log.Error("Failed to parse template: %s", err)
		return err
	} else {
		if err := tmpl.Execute(out, data); err != nil {
			log.Error("Failed to execute template: %s", err)
			return err
		}
	}
	return nil
}

func ResponseToJson(resp *http.Response, err error) (interface{}, error) {
	if err != nil {
		return nil, err
	}

	data := JsonDecode(resp.Body)
	if resp.StatusCode == 400 {
		if val, ok := data.(map[string]interface{})["errorMessages"]; ok {
			for _, errMsg := range val.([]interface{}) {
				log.Error("%s", errMsg)
			}
		}
	}

	return data, nil
}

func JsonDecode(io io.Reader) interface{} {
	content, err := ioutil.ReadAll(io)
	var data interface{}
	err = json.Unmarshal(content, &data)
	if err != nil {
		log.Error("JSON Parse Error: %s from %s", err, content)
	}
	return data
}

func JsonEncode(data interface{}) (string, error) {
	buffer := bytes.NewBuffer(make([]byte, 0))
	enc := json.NewEncoder(buffer)

	err := enc.Encode(data)
	if err != nil {
		log.Error("Failed to encode data %s: %s", data, err)
		return "", err
	}
	return buffer.String(), nil
}

func JsonWrite(file string, data interface{}) {
	fh, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	defer fh.Close()
	if err != nil {
		log.Error("Failed to open %s: %s", file, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(fh)
	enc.Encode(data)
}

func PromptYN(prompt string, yes bool) bool {
	reader := bufio.NewReader(os.Stdin)
	if !yes {
		prompt = fmt.Sprintf("%s [y/N]: ", prompt)
	} else {
		prompt = fmt.Sprintf("%s [Y/n]: ", prompt)
	}

	fmt.Printf("%s", prompt)
	text, _ := reader.ReadString('\n')
	ans := strings.ToLower(strings.TrimRight(text, "\n"))
	if ans == "" {
		return yes
	}
	if strings.HasPrefix(ans, "y") {
		return true
	}
	return false
}

func Prompt(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	prompt = fmt.Sprintf("%s: ", prompt)
	fmt.Printf("%s", prompt)
	out, _ := reader.ReadString('\n')
	return strings.TrimSpace(out)
}

func PromptWithDefault(prompt string, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)
	prompt = fmt.Sprintf("%s [%s]: ", prompt, defaultValue)
	fmt.Printf("%s", prompt)
	out, _ := reader.ReadString('\n')
	if len(strings.TrimSpace(out)) == 0 {
		return defaultValue
	} else {
		return strings.TrimSpace(out)
	}
}

func ParseYaml(file string, opts *map[string]interface{}) {
	if fh, err := ioutil.ReadFile(file); err == nil {
		log.Debug("Found Config file: %s", file)
		yaml.Unmarshal(fh, opts)
		fixed, _ := YamlFixup(*opts)
		*opts = fixed.(map[string]interface{})
	}
}

func YamlFixup(data interface{}) (interface{}, error) {
	switch d := data.(type) {
	case map[interface{}]interface{}:
		// need to copy this map into a string map so json can encode it
		copy := make(map[string]interface{})
		for key, val := range d {
			switch k := key.(type) {
			case string:
				if fixed, err := YamlFixup(val); err != nil {
					return nil, err
				} else if fixed != nil {
					copy[k] = fixed
				}
			default:
				err := fmt.Errorf("YAML: key %s is type '%T', require 'string'", key, k)
				log.Error("%s", err)
				return nil, err
			}
		}
		return copy, nil
	case map[string]interface{}:
		copy := make(map[string]interface{})
		for k, v := range d {
			if fixed, err := YamlFixup(v); err != nil {
				return nil, err
			} else if fixed != nil {
				copy[k] = fixed
			}
		}
		return copy, nil
	case []interface{}:
		copy := make([]interface{}, 0, len(d))
		for _, val := range d {
			if fixed, err := YamlFixup(val); err != nil {
				return nil, err
			} else if fixed != nil {
				copy = append(copy, fixed)
			}
		}
		return copy, nil
	case string:
		if d == "" || d == "\n" {
			return nil, nil
		}
		return d, nil
	default:
		return d, nil
	}
}

func Mkdir(dir string) error {
	if stat, err := os.Stat(dir); err != nil && !os.IsNotExist(err) {
		log.Error("Failed to stat %s: %s", dir, err)
		return err
	} else if err == nil && !stat.IsDir() {
		err := fmt.Errorf("%s exists and is not a directory!", dir)
		log.Error("%s", err)
		return err
	} else {
		// dir does not exist, so try to create it
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Error("Failed to mkdir -p %s: %s", dir, err)
			return err
		}
	}
	return nil
}
