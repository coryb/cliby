package cliby

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/coryb/cliby/util"
	"github.com/fatih/camelcase"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/coryb/yaml.v2"
	"gopkg.in/op/go-logging.v1"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"time"
	"unicode"
)

var log = logging.MustGetLogger("cliby")

type Exit struct{ Code int }

type Cli struct {
	cookieFile string
	ua         *http.Client
	commands   map[string]func() error
	defaults   interface{}
	options    interface{}
	templates  map[string]string
	name       string
}

type Options struct {
	ConfigFile string `json:"config-file"`
}

func New(name string) *Cli {
	homedir := os.Getenv("HOME")

	cli := &Cli{
		cookieFile: fmt.Sprintf("%s/.%s.d/cookies.js", homedir, name),
		ua:         &http.Client{},
		name:       name,
		commands:   make(map[string]func() error),
	}

	return cli
}

func (c *Cli) Name() string {
	return c.name
}

func (c *Cli) GetDefaults() interface{} {
	return c.defaults
}

func (c *Cli) SetDefaults(val interface{}) {
	c.defaults = val
}

func (c *Cli) NewOptions() interface{} {
	return Options{}
}

func (c *Cli) GetOptions() interface{} {
	return c.options
}

func (c *Cli) SetOptions(val interface{}) {
	c.options = val
}

func (c *Cli) SetCommands(commands map[string]func() error) {
	c.commands = commands
}

func (c *Cli) GetCommand(command string) func() error {
	if fn, ok := c.commands[command]; !ok {
		return nil
	} else {
		return fn
	}
}

func (c *Cli) SetTemplates(templates map[string]string) {
	c.templates = templates
}

func (c *Cli) GetTemplate(template string) string {
	if fn, ok := c.templates[template]; !ok {
		return ""
	} else {
		return fn
	}
}

func (c *Cli) GetHttpClient() *http.Client {
	return c.ua
}

func (c *Cli) SetHttpClient(client *http.Client) {
	c.ua = client
}

func (c *Cli) SetCookieFile(file string) {
	c.cookieFile = file
}

func (c *Cli) CommandLine() *kingpin.Application {
	log.Errorf("CommandLine not implemented")
	panic(Exit{1})
}

var LOG_FORMAT = "%{color}%{time:2006-01-02T15:04:05.000Z07:00} %{level:-5s} [%{shortfile}]%{color:reset} %{message}"

func InitLogging() {
	logBackend := logging.NewLogBackend(os.Stderr, "", 0)
	logging.SetBackend(
		logging.NewBackendFormatter(
			logBackend,
			logging.MustStringFormatter(LOG_FORMAT),
		),
	)
	logging.SetLevel(logging.NOTICE, "")
}

func RunCommand(i Interface, command string) error {
	fn := i.GetCommand(command)
	if fn != nil {
		return fn()
	}
	i.CommandLine().Usage([]string{})
	if command == "" {
		for _, arg := range os.Args[1:] {
			if arg[0] != '-' {
				command = arg
				break
			}
		}
	}
	return fmt.Errorf("Command %s Unknown", command)
}

func ProcessAllOptions(i Interface) string {
	app := i.CommandLine()
	app.Terminate(func(status int) {
		for _, arg := range os.Args {
			if arg == "-h" || arg == "--help" || len(os.Args) == 1 {
				panic(Exit{0})
			}
		}
		panic(Exit{status})
	})
	command, err := app.Parse(os.Args[1:])
	if err != nil {
		log.Errorf("%s", err)
		if command == "" {
			for _, arg := range os.Args[1:] {
				if arg[0] != '-' {
					command = arg
					break
				}
			}
		}
		app.Usage([]string{command})

		panic(Exit{1})
	}
	os.Setenv(fmt.Sprintf("%s_OPERATION", strings.ToUpper(i.Name())), command)

	// at this point Config is populated with with defaults
	processConfigs(i)
	return command
}

// func (c *Cli) ProcessAllOptions() string {
// 	if err := c.Options.ProcessAll(os.Args[1:]); err != nil {
// 		log.Errorf("%s", err)
// 		c.PrintUsage(false)
// 	}
// 	return c.processConfigs()

// }

func processConfigs(i Interface) {
	defaults := i.GetDefaults()
	if defaults == nil {
		defaults = i.NewOptions()
	}
	options := i.GetOptions()
	if options == nil {
		options = i.NewOptions()
	}

	var configFile string
Outer:
	for _, name := range []string{"ConfigFile", "config-file"} {
		for _, collection := range []interface{}{options, defaults} {
			configFile = getKeyString(collection, name)
			if configFile != "" {
				break Outer
			}
		}
	}
	if configFile == "" {
		configFile = fmt.Sprintf(".%s.d/config.yml", i.Name())
	}

	i.SetOptions(options)

	LoadConfigs(i, configFile)

	dv := reflect.ValueOf(defaults)
	ov := reflect.ValueOf(options)

	log.Debugf("defaults: %#v  options: %#v", defaults, i.GetOptions())
	log.Debugf("Setting Config from Defaults")
	MergeStructs(ov, dv)
	i.SetOptions(ov.Interface())
	populateEnv(i)
}

// func (c *Cli) SetEditing(dflt bool) {
// 	log.Debugf("Default Editing: %t", dflt)
// 	if dflt {
// 		if val, ok := c.Opts["noedit"].(bool); ok && val {
// 			log.Debugf("Setting edit = false")
// 			c.Opts["edit"] = false
// 		} else {
// 			log.Debugf("Setting edit = true")
// 			c.Opts["edit"] = true
// 		}
// 	} else {
// 		if _, ok := c.Opts["edit"].(bool); !ok {
// 			log.Debugf("Setting edit = %t", dflt)
// 			c.Opts["edit"] = dflt
// 		}
// 	}
// }

func populateEnv(iface Interface) {
	options := reflect.ValueOf(iface.GetOptions())
	if options.Kind() == reflect.Ptr {
		options = reflect.ValueOf(options.Elem().Interface())
	}
	if options.Kind() == reflect.Struct {
		for i := 0; i < options.NumField(); i++ {
			name := strings.Join(camelcase.Split(options.Type().Field(i).Name), "_")
			envName := fmt.Sprintf("%s_%s", strings.ToUpper(iface.Name()), strings.ToUpper(name))

			envName = strings.Map(func(r rune) rune {
				if unicode.IsDigit(r) || unicode.IsLetter(r) {
					return r
				}
				return '_'
			}, envName)
			var val string
			switch t := options.Field(i).Interface().(type) {
			case string:
				val = t
			case int, int8, int16, int32, int64:
				val = fmt.Sprintf("%d", t)
			case float32, float64:
				val = fmt.Sprintf("%f", t)
			case bool:
				val = fmt.Sprintf("%t", t)
			default:
				val = fmt.Sprintf("%v", t)
			}
			os.Setenv(envName, val)
		}
	}
}

func LoadConfigs(iface Interface, configFile string) {
	populateEnv(iface)

	paths := util.FindParentPaths(configFile)
	// prepend
	paths = append([]string{fmt.Sprintf("/etc/%s.yml", iface.Name())}, paths...)

	// iterate paths in reverse
	for i := len(paths) - 1; i >= 0; i-- {
		file := paths[i]
		if stat, err := os.Stat(file); err == nil {
			tmp := iface.NewOptions()
			// check to see if config file is exectuable
			if stat.Mode()&0111 == 0 {
				log.Debugf("Loading config %s", file)
				if fh, err := ioutil.ReadFile(file); err == nil {
					yaml.Unmarshal(fh, tmp)
					if reflect.ValueOf(tmp).Kind() == reflect.Map {
						tmp, _ = util.YamlFixup(tmp)
					}
				}
			} else {
				log.Debugf("Found Executable Config file: %s", file)
				// it is executable, so run it and try to parse the output
				cmd := exec.Command(file)
				stdout := bytes.NewBufferString("")
				cmd.Stdout = stdout
				cmd.Stderr = bytes.NewBufferString("")
				if err := cmd.Run(); err != nil {
					log.Errorf("%s is exectuable, but it failed to execute: %s\n%s", file, err, cmd.Stderr)
					panic(Exit{1})
				}
				yaml.Unmarshal(stdout.Bytes(), &tmp)
			}

			nv := reflect.ValueOf(tmp)
			ov := reflect.ValueOf(iface.GetOptions())

			log.Debugf("Setting Config from %s", file)
			MergeStructs(ov, nv)
			iface.SetOptions(ov.Interface())
			populateEnv(iface)
		}
	}
}

func MergeStructs(ov, nv reflect.Value) {
	if ov.Kind() == reflect.Ptr {
		ov = ov.Elem()
	}
	if nv.Kind() == reflect.Ptr {
		nv = nv.Elem()
	}
	if ov.Kind() == reflect.Map && nv.Kind() == reflect.Map {
		MergeMaps(ov, nv)
		return
	}
	if !ov.IsValid() || !nv.IsValid() {
		return
	}
	for i := 0; i < nv.NumField(); i++ {
		if reflect.DeepEqual(ov.Field(i).Interface(), reflect.Zero(ov.Field(i).Type()).Interface()) && !reflect.DeepEqual(ov.Field(i).Interface(), nv.Field(i).Interface()) {
			log.Debugf("Setting %s to %#v", nv.Type().Field(i).Name, nv.Field(i).Interface())
			ov.Field(i).Set(nv.Field(i))
		} else {
			switch ov.Field(i).Kind() {
			case reflect.Map:
				if nv.Field(i).Len() > 0 {
					log.Debugf("merging: %v with %v", ov.Field(i), nv.Field(i))
					MergeMaps(ov.Field(i), nv.Field(i))
				}
			case reflect.Slice:
				if nv.Field(i).Len() > 0 {
					log.Debugf("merging: %v with %v", ov.Field(i), nv.Field(i))
					if ov.Field(i).CanSet() {
						if ov.Field(i).Len() == 0 {
							ov.Field(i).Set(nv.Field(i))
						} else {
							log.Debugf("merging: %v with %v", ov.Field(i), nv.Field(i))
							ov.Field(i).Set(MergeArrays(ov.Field(i), nv.Field(i)))
						}
					}

				}
			case reflect.Array:
				if nv.Field(i).Len() > 0 {
					log.Debugf("merging: %v with %v", ov.Field(i), nv.Field(i))
					ov.Field(i).Set(MergeArrays(ov.Field(i), nv.Field(i)))
				}
			}
		}
	}
}

func MergeMaps(ov, nv reflect.Value) {
	for _, key := range nv.MapKeys() {
		if !ov.MapIndex(key).IsValid() {
			log.Debugf("Setting %v to %#v", key.Interface(), nv.MapIndex(key).Interface())
			ov.SetMapIndex(key, nv.MapIndex(key))
		} else {
			ovi := reflect.ValueOf(ov.MapIndex(key).Interface())
			nvi := reflect.ValueOf(nv.MapIndex(key).Interface())
			switch ovi.Kind() {
			case reflect.Map:
				log.Debugf("merging: %v with %v", ovi.Interface(), nvi.Interface())
				MergeMaps(ovi, nvi)
			case reflect.Slice:
				log.Debugf("merging: %v with %v", ovi.Interface(), nvi.Interface())
				ov.SetMapIndex(key, MergeArrays(ovi, nvi))
			case reflect.Array:
				log.Debugf("merging: %v with %v", ovi.Interface(), nvi.Interface())
				ov.SetMapIndex(key, MergeArrays(ovi, nvi))
			}
		}
	}
}

func MergeArrays(ov, nv reflect.Value) reflect.Value {
Outer:
	for ni := 0; ni < nv.Len(); ni++ {
		niv := nv.Index(ni)
		for oi := 0; oi < ov.Len(); oi++ {
			oiv := ov.Index(oi)
			if reflect.DeepEqual(niv.Interface(), oiv.Interface()) {
				continue Outer
			}
		}
		log.Debugf("appending %v to %v", niv.Interface(), ov)
		ov = reflect.Append(ov, niv)
	}
	return ov
}

func (c *Cli) saveCookies(cookies []*http.Cookie) {
	// expiry in one week from now
	expiry := time.Now().Add(24 * 7 * time.Hour)
	for _, cookie := range cookies {
		cookie.Expires = expiry
	}

	if currentCookies := c.loadCookies(); currentCookies != nil {
		currentCookiesByName := make(map[string]*http.Cookie)
		for _, cookie := range currentCookies {
			currentCookiesByName[cookie.Name] = cookie
		}

		for _, cookie := range cookies {
			currentCookiesByName[cookie.Name] = cookie
		}

		mergedCookies := make([]*http.Cookie, 0, len(currentCookiesByName))
		for _, v := range currentCookiesByName {
			mergedCookies = append(mergedCookies, v)
		}
		util.JsonWrite(c.cookieFile, mergedCookies)
	} else {
		util.JsonWrite(c.cookieFile, cookies)
	}
}

func (c *Cli) loadCookies() []*http.Cookie {
	bytes, err := ioutil.ReadFile(c.cookieFile)
	if err != nil && os.IsNotExist(err) {
		// dont load cookies if the file does not exist
		return nil
	}
	if err != nil {
		log.Errorf("Failed to open %s: %s", c.cookieFile, err)
		panic(Exit{1})
	}
	cookies := make([]*http.Cookie, 0)
	err = json.Unmarshal(bytes, &cookies)
	if err != nil {
		log.Errorf("Failed to parse json from file %s: %s", c.cookieFile, err)
	}
	log.Debugf("Loading Cookies: %s", cookies)
	return cookies
}

func (c *Cli) initCookies(uri string) {
	if c.ua.Jar == nil {
		url, _ := url.Parse(uri)
		jar, _ := cookiejar.New(nil)
		c.ua.Jar = jar
		c.ua.Jar.SetCookies(url, c.loadCookies())
	}
}

func (c *Cli) Post(uri string, content string) (*http.Response, error) {
	c.initCookies(uri)
	return c.makeRequestWithContent("POST", uri, content, "application/json")
}

func (c *Cli) Put(uri string, content string) (*http.Response, error) {
	c.initCookies(uri)
	return c.makeRequestWithContent("PUT", uri, content, "application/json")
}

func (c *Cli) Delete(uri string, content string) (*http.Response, error) {
	c.initCookies(uri)
	return c.makeRequestWithContent("DELETE", uri, content, "application/json")
}

func (c *Cli) PostXML(uri string, content string) (*http.Response, error) {
	c.initCookies(uri)
	return c.makeRequestWithContent("POST", uri, content, "application/xml")
}

func (c *Cli) PutXML(uri string, content string) (*http.Response, error) {
	c.initCookies(uri)
	return c.makeRequestWithContent("PUT", uri, content, "application/xml")
}

func (c *Cli) makeRequestWithContent(method string, uri string, content string, contentType string) (*http.Response, error) {
	buffer := bytes.NewBufferString(content)
	req, _ := http.NewRequest(method, uri, buffer)
	req.Header.Set("Content-Type", contentType)

	log.Debugf("%s %s", req.Method, req.URL.String())
	if log.IsEnabledFor(logging.DEBUG) {
		out, _ := httputil.DumpRequestOut(req, true)
		log.Debugf("%s", out)
	}

	if resp, err := c.makeRequest(req); err != nil {
		return nil, err
	} else {
		if resp.StatusCode == 401 {
			if err := c.Login(); err != nil {
				return nil, err
			}
			req, _ = http.NewRequest(method, uri, bytes.NewBufferString(content))
			return c.makeRequest(req)
		}
		return resp, err
	}
}

func (c *Cli) Get(uri string) (*http.Response, error) {
	c.initCookies(uri)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		log.Errorf("Invalid Request: %s", uri)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	log.Debugf("%s %s", req.Method, req.URL.String())
	if log.IsEnabledFor(logging.DEBUG) {
		logBuffer := bytes.NewBuffer(make([]byte, 0))
		req.Write(logBuffer)
		log.Debugf("%s", logBuffer)
	}

	if resp, err := c.makeRequest(req); err != nil {
		return nil, err
	} else {
		if resp.StatusCode == 401 {
			if err := c.Login(); err != nil {
				return nil, err
			}
			return c.makeRequest(req)
		}
		return resp, err
	}
}

func (c *Cli) makeRequest(req *http.Request) (resp *http.Response, err error) {
	if resp, err = c.ua.Do(req); err != nil {
		return nil, err
	} else {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 && resp.StatusCode != 401 {
			log.Debugf("response status: %s", resp.Status)
		}

		runtime.SetFinalizer(resp, func(r *http.Response) {
			r.Body.Close()
		})

		if _, ok := resp.Header["Set-Cookie"]; ok {
			c.saveCookies(resp.Cookies())
		}
	}
	return resp, nil
}

// func (c *Cli) GetTemplate(name string) string {
// 	if override, ok := c.Opts["template"].(string); ok {
// 		if _, err := os.Stat(override); err == nil {
// 			return util.ReadFile(override)
// 		} else {
// 			if file, err := util.FindClosestParentPath(fmt.Sprintf(".%s.d/templates/%s", c.Name, override)); err == nil {
// 				return util.ReadFile(file)
// 			}
// 			if dflt, ok := c.Templates[override]; ok {
// 				return dflt
// 			}
// 		}
// 	}
// 	if file, err := util.FindClosestParentPath(fmt.Sprintf(".%s.d/templates/%s", c.Name, name)); err != nil {
// 		return c.Templates[name]
// 	} else {
// 		return util.ReadFile(file)
// 	}
// }

type NoChangesFound struct{}

func (f NoChangesFound) Error() string {
	return "No changes found, aborting"
}

// func (c *Cli) editTemplate(template string, tmpFilePrefix string, templateData map[string]interface{}, templateProcessor func(string) error) error {

// 	tmpdir := fmt.Sprintf("%s/.%s.d/tmp", os.Getenv("HOME"), c.Name)
// 	if err := util.Mkdir(tmpdir); err != nil {
// 		return err
// 	}

// 	fh, err := ioutil.TempFile(tmpdir, tmpFilePrefix)
// 	if err != nil {
// 		log.Errorf("Failed to make temp file in %s: %s", tmpdir, err)
// 		return err
// 	}
// 	defer fh.Close()

// 	tmpFileName := fmt.Sprintf("%s.yml", fh.Name())
// 	if err := os.Rename(fh.Name(), tmpFileName); err != nil {
// 		log.Errorf("Failed to rename %s to %s: %s", fh.Name(), fmt.Sprintf("%s.yml", fh.Name()), err)
// 		return err
// 	}
// 	defer func() {
// 		os.Remove(tmpFileName)
// 	}()

// 	err = util.RunTemplate(template, templateData, fh)
// 	if err != nil {
// 		return err
// 	}

// 	fh.Close()

// 	editor, ok := c.Opts["editor"].(string)
// 	if !ok {
// 		editor = os.Getenv(fmt.Sprintf("%s_EDITOR", strings.ToUpper(c.name)))
// 		if editor == "" {
// 			editor = os.Getenv("EDITOR")
// 			if editor == "" {
// 				editor = "vim"
// 			}
// 		}
// 	}

// 	editing := c.getOptBool("edit", true)

// 	tmpFileNameOrig := fmt.Sprintf("%s.orig", tmpFileName)
// 	util.CopyFile(tmpFileName, tmpFileNameOrig)
// 	defer func() {
// 		os.Remove(tmpFileNameOrig)
// 	}()

// 	for true {
// 		if editing {
// 			shell, _ := shellquote.Split(editor)
// 			shell = append(shell, tmpFileName)
// 			log.Debugf("Running: %#v", shell)
// 			cmd := exec.Command(shell[0], shell[1:]...)
// 			cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
// 			if err := cmd.Run(); err != nil {
// 				log.Errorf("Failed to edit template with %s: %s", editor, err)
// 				if util.PromptYN("edit again?", true) {
// 					continue
// 				}
// 				return err
// 			}

// 			diff := exec.Command("diff", "-q", tmpFileNameOrig, tmpFileName)
// 			// if err == nil then diff found no changes
// 			if err := diff.Run(); err == nil {
// 				return NoChangesFound{}
// 			}
// 		}

// 		edited := make(map[string]interface{})
// 		if fh, err := ioutil.ReadFile(tmpFileName); err != nil {
// 			log.Errorf("Failed to read tmpfile %s: %s", tmpFileName, err)
// 			if editing && util.PromptYN("edit again?", true) {
// 				continue
// 			}
// 			return err
// 		} else {
// 			if err := yaml.Unmarshal(fh, &edited); err != nil {
// 				log.Errorf("Failed to parse YAML: %s", err)
// 				if editing && util.PromptYN("edit again?", true) {
// 					continue
// 				}
// 				return err
// 			}
// 		}

// 		if fixed, err := util.YamlFixup(edited); err != nil {
// 			return err
// 		} else {
// 			edited = fixed.(map[string]interface{})
// 		}

// 		// if you want to abort editing a jira issue then
// 		// you can add the "abort: true" flag to the document
// 		// and we will abort now
// 		if val, ok := edited["abort"].(bool); ok && val {
// 			log.Infof("abort flag found in template, quiting")
// 			return fmt.Errorf("abort flag found in template, quiting")
// 		}

// 		if _, ok := templateData["meta"]; ok {
// 			mf := templateData["meta"].(map[string]interface{})["fields"]
// 			if f, ok := edited["fields"].(map[string]interface{}); ok {
// 				for k := range f {
// 					if _, ok := mf.(map[string]interface{})[k]; !ok {
// 						err := fmt.Errorf("Field %s is not editable", k)
// 						log.Errorf("%s", err)
// 						if editing && util.PromptYN("edit again?", true) {
// 							continue
// 						}
// 						return err
// 					}
// 				}
// 			}
// 		}

// 		json, err := util.JsonEncode(edited)
// 		if err != nil {
// 			return err
// 		}

// 		if err := templateProcessor(json); err != nil {
// 			log.Errorf("%s", err)
// 			if editing && util.PromptYN("edit again?", true) {
// 				continue
// 			}
// 		}
// 		return nil
// 	}
// 	return nil
// }

func (c *Cli) Browse(uri string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", uri).Run()
	} else if runtime.GOOS == "linux" {
		return exec.Command("xdg-open", uri).Run()
	}
	return nil
}

func getKeyString(data interface{}, key string) string {
	val := reflect.ValueOf(data)
	if !val.IsValid() {
		return ""
	}
	if val.Kind() == reflect.Ptr {
		val = reflect.ValueOf(val.Elem().Interface())
	}
	var result reflect.Value
	log.Debugf("looking up %s in %s %#v", key, val.Kind(), data)
	switch val.Kind() {
	// case reflect.Ptr:
	// 	log.Debugf("looking up %s in %s %#v", key, val.Elem().Kind(), data)
	// 	if val.Elem().Kind() == reflect.Ptr {
	// 		return ""
	// 	}
	// 	return getKeyString(val.Elem().Interface(), key)
	case reflect.Map:
		result = val.MapIndex(reflect.ValueOf(key))
	case reflect.Struct:
		result = val.FieldByName(key)
	default:
		return ""
	}
	if result.IsValid() {
		log.Debugf("lookup of %s in %v, found %v (%s)", key, data, result.Interface(), result.Kind())
		if val, ok := result.Interface().(string); ok {
			log.Debugf("returning %s", val)
			return val
		}
	}
	// if result.Kind() == reflect.String {
	// 	return result.Interface().(string)
	// }
	return ""
}

func setKeyString(data interface{}, key string, value interface{}) {
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = reflect.ValueOf(val.Elem().Interface())
	}
	log.Debugf("Setting %s in %#v to %#v", key, val.Interface(), value)
	var result reflect.Value
	// log.Debugf("type: %s", val.Kind())
	// log.Debugf("type: %s", reflect.TypeOf(val.Interface()).Kind())
	switch val.Kind() {
	case reflect.Map:
		keyValue := reflect.ValueOf(key)
		log.Debugf("Setting map index %s to %#v", keyValue.Interface(), value)
		val.SetMapIndex(keyValue, reflect.ValueOf(value))
	case reflect.Struct:
		result = val.FieldByName(key)
		if result.IsValid() {
			result.Set(reflect.ValueOf(value))
		}
	}
	log.Debugf("New val: %#v", val.Interface())
}

func (c *Cli) Login() error {
	return fmt.Errorf("Login not implemented")
}

// func (c *Cli) ExportTemplates() error {
// 	dir := c.Opts["directory"].(string)
// 	if err := util.Mkdir(dir); err != nil {
// 		return err
// 	}

// 	for name, template := range c.Templates {
// 		if wanted, ok := c.Opts["template"]; ok && wanted != name {
// 			continue
// 		}
// 		templateFile := fmt.Sprintf("%s/%s", dir, name)
// 		if _, err := os.Stat(templateFile); err == nil {
// 			log.Warning("Skipping %s, already exists", templateFile)
// 			continue
// 		}
// 		if fh, err := os.OpenFile(templateFile, os.O_WRONLY|os.O_CREATE, 0644); err != nil {
// 			log.Errorf("Failed to open %s for writing: %s", templateFile, err)
// 			return err
// 		} else {
// 			defer fh.Close()
// 			log.Noticef("Creating %s", templateFile)
// 			fh.Write([]byte(template))
// 		}
// 	}
// 	return nil
// }
