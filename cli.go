package cliby

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/coryb/optigo"
	"github.com/kballard/go-shellquote"
	"github.com/op/go-logging"
	"gopkg.in/coryb/yaml.v2"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	// "net/url"
	"github.com/coryb/cliby/util"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

var log = logging.MustGetLogger("cliby")

type Cli struct {
	CookieFile     string
	UA             *http.Client
	Usage          func() string
	Commands       map[string]func() error
	CommandAliases map[string]string
	Opts           map[string]interface{}
	Args           []string
	Options        optigo.OptionParser
	Templates      map[string]string
	Name           string
}

func New(name string) *Cli {
	homedir := os.Getenv("HOME")

	cli := &Cli{
		CookieFile: fmt.Sprintf("%s/.%s.d/cookies.js", homedir, name),
		UA:         &http.Client{},
		Name:       name,
		Opts: map[string]interface{}{
			"config-file": fmt.Sprintf(".%s.d/config.yml", name),
		},
		Commands:       make(map[string]func() error),
		CommandAliases: make(map[string]string),
	}

	return cli
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

func (c *Cli) PrintUsage(ok bool) {
	printer := fmt.Printf
	if !ok {
		printer = func(format string, args ...interface{}) (int, error) {
			return fmt.Fprintf(os.Stderr, format, args...)
		}
		defer func() {
			os.Exit(1)
		}()
	} else {
		defer func() {
			os.Exit(0)
		}()
	}
	printer(c.Usage())
}

func (c *Cli) RunCommand(command string) error {
	if fn, ok := c.Commands[command]; !ok {
		log.Error("Command %s Unknown", command)
		c.PrintUsage(false)
		return nil // PrintUsage will call os.Exit
	} else {
		return fn()
	}
}

func (c *Cli) ProcessOptions() string {
	c.Options.Results = c.Opts
	if err := c.Options.ProcessSome(os.Args[1:]); err != nil {
		log.Error("%s", err)
		c.PrintUsage(false)
	}
	return c.processConfigs()
}

func (c *Cli) ProcessAllOptions() string {
	c.Options.Results = c.Opts
	if err := c.Options.ProcessAll(os.Args[1:]); err != nil {
		log.Error("%s", err)
		c.PrintUsage(false)
	}
	return c.processConfigs()
}

func (c *Cli) processConfigs() string {
	c.Args = c.Options.Args

	var command string
	if len(c.Args) > 0 {
		if _, ok := c.Commands[c.Args[0]]; ok {
			command = c.Args[0]
			c.Args = c.Args[1:]
		} else if len(c.Args) > 1 {
			// look at second arg for cases like: arg1 doesSomethingTo arg2
			// where the command is actually doesSomethingTo
			if _, ok := c.Commands[c.Args[1]]; ok {
				command = c.Args[1]
				c.Args = append(c.Args[:1], c.Args[2:]...)
			}
		}
	}

	if command == "" && len(c.Args) > 0 {
		command = c.Args[0]
		c.Args = c.Args[1:]
	}

	os.Setenv(fmt.Sprintf("%s_OPERATION", strings.ToUpper(c.Name)), command)
	c.loadConfigs()

	// check to see if it was set in the configs:
	if value, ok := c.Opts["command"].(string); ok {
		command = value
	} else if _, ok := c.Commands[command]; !ok || command == "" {
		c.Args = append([]string{command}, c.Args...)
		command = "default"
	}

	return command
}

func (c *Cli) SetEditing(dflt bool) {
	log.Debug("Default Editing: %t", dflt)
	if dflt {
		if val, ok := c.Opts["noedit"].(bool); ok && val {
			log.Debug("Setting edit = false")
			c.Opts["edit"] = false
		} else {
			log.Debug("Setting edit = true")
			c.Opts["edit"] = true
		}
	} else {
		if _, ok := c.Opts["edit"].(bool); !ok {
			log.Debug("Setting edit = %t", dflt)
			c.Opts["edit"] = dflt
		}
	}
}

func (c *Cli) populateEnv() {
	for k, v := range c.Opts {
		envName := fmt.Sprintf("%s_%s", strings.ToUpper(c.Name), strings.ToUpper(k))
		var val string
		switch t := v.(type) {
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

func (c *Cli) loadConfigs() {
	c.populateEnv()
	var configFile string
	if val, ok := c.Opts["config-file"].(string); ok && val != "" {
		configFile = val
	}

	paths := util.FindParentPaths(configFile)
	// prepend
	paths = append([]string{fmt.Sprintf("/etc/%s.yml", c.Name)}, paths...)

	// iterate paths in reverse
	for i := len(paths) - 1; i >= 0; i-- {
		file := paths[i]
		if stat, err := os.Stat(file); err == nil {
			tmp := make(map[string]interface{})
			// check to see if config file is exectuable
			if stat.Mode()&0111 == 0 {
				util.ParseYaml(file, tmp)
			} else {
				log.Debug("Found Executable Config file: %s", file)
				// it is executable, so run it and try to parse the output
				cmd := exec.Command(file)
				stdout := bytes.NewBufferString("")
				cmd.Stdout = stdout
				cmd.Stderr = bytes.NewBufferString("")
				if err := cmd.Run(); err != nil {
					log.Error("%s is exectuable, but it failed to execute: %s\n%s", file, err, cmd.Stderr)
					os.Exit(1)
				}
				yaml.Unmarshal(stdout.Bytes(), &tmp)
			}
			for k, v := range tmp {
				if _, ok := c.Opts[k]; !ok {
					log.Debug("Setting %q to %#v from %s", k, v, file)
					c.Opts[k] = v
				}
			}
			c.populateEnv()
		}
	}
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
		util.JsonWrite(c.CookieFile, mergedCookies)
	} else {
		util.JsonWrite(c.CookieFile, cookies)
	}
}

func (c *Cli) loadCookies() []*http.Cookie {
	bytes, err := ioutil.ReadFile(c.CookieFile)
	if err != nil && os.IsNotExist(err) {
		// dont load cookies if the file does not exist
		return nil
	}
	if err != nil {
		log.Error("Failed to open %s: %s", c.CookieFile, err)
		os.Exit(1)
	}
	cookies := make([]*http.Cookie, 0)
	err = json.Unmarshal(bytes, &cookies)
	if err != nil {
		log.Error("Failed to parse json from file %s: %s", c.CookieFile, err)
	}
	log.Debug("Loading Cookies: %s", cookies)
	return cookies
}

func (c *Cli) initCookies(uri string) {
	if c.UA.Jar == nil {
		url, _ := url.Parse(uri)
		jar, _ := cookiejar.New(nil)
		c.UA.Jar = jar
		c.UA.Jar.SetCookies(url, c.loadCookies())
	}
}

func (c *Cli) Post(uri string, content string) (*http.Response, error) {
	c.initCookies(uri)
	return c.makeRequestWithContent("POST", uri, content)
}

func (c *Cli) Put(uri string, content string) (*http.Response, error) {
	c.initCookies(uri)
	return c.makeRequestWithContent("PUT", uri, content)
}

func (c *Cli) makeRequestWithContent(method string, uri string, content string) (*http.Response, error) {
	buffer := bytes.NewBufferString(content)
	req, _ := http.NewRequest(method, uri, buffer)

	log.Info("%s %s", req.Method, req.URL.String())
	if log.IsEnabledFor(logging.DEBUG) {
		logBuffer := bytes.NewBuffer(make([]byte, 0, len(content)))
		req.Write(logBuffer)
		log.Debug("%s", logBuffer)
		// need to recreate the buffer since the offset is now at the end
		// need to be able to rewind the buffer offset, dont know how yet
		req, _ = http.NewRequest(method, uri, bytes.NewBufferString(content))
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
	req, _ := http.NewRequest("GET", uri, nil)
	log.Info("%s %s", req.Method, req.URL.String())
	if log.IsEnabledFor(logging.DEBUG) {
		logBuffer := bytes.NewBuffer(make([]byte, 0))
		req.Write(logBuffer)
		log.Debug("%s", logBuffer)
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
	req.Header.Set("Content-Type", "application/json")
	if resp, err = c.UA.Do(req); err != nil {
		log.Error("Failed to %s %s: %s", req.Method, req.URL.String(), err)
		return nil, err
	} else {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 && resp.StatusCode != 401 {
			log.Error("response status: %s", resp.Status)
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

func (c *Cli) getTemplate(name string) string {
	if override, ok := c.Opts["template"].(string); ok {
		if _, err := os.Stat(override); err == nil {
			return util.ReadFile(override)
		} else {
			if file, err := util.FindClosestParentPath(fmt.Sprintf(".%s.d/templates/%s", c.Name, override)); err == nil {
				return util.ReadFile(file)
			}
			if dflt, ok := c.Templates[override]; ok {
				return dflt
			}
		}
	}
	if file, err := util.FindClosestParentPath(fmt.Sprintf(".%s.d/templates/%s", c.Name, name)); err != nil {
		return c.Templates[name]
	} else {
		return util.ReadFile(file)
	}
}

type NoChangesFound struct{}

func (f NoChangesFound) Error() string {
	return "No changes found, aborting"
}

func (c *Cli) editTemplate(template string, tmpFilePrefix string, templateData map[string]interface{}, templateProcessor func(string) error) error {

	tmpdir := fmt.Sprintf("%s/.%s.d/tmp", os.Getenv("HOME"), c.Name)
	if err := util.Mkdir(tmpdir); err != nil {
		return err
	}

	fh, err := ioutil.TempFile(tmpdir, tmpFilePrefix)
	if err != nil {
		log.Error("Failed to make temp file in %s: %s", tmpdir, err)
		return err
	}
	defer fh.Close()

	tmpFileName := fmt.Sprintf("%s.yml", fh.Name())
	if err := os.Rename(fh.Name(), tmpFileName); err != nil {
		log.Error("Failed to rename %s to %s: %s", fh.Name(), fmt.Sprintf("%s.yml", fh.Name()), err)
		return err
	}
	defer func() {
		os.Remove(tmpFileName)
	}()

	err = util.RunTemplate(template, templateData, fh)
	if err != nil {
		return err
	}

	fh.Close()

	editor, ok := c.Opts["editor"].(string)
	if !ok {
		editor = os.Getenv(fmt.Sprintf("%s_EDITOR", strings.ToUpper(c.Name)))
		if editor == "" {
			editor = os.Getenv("EDITOR")
			if editor == "" {
				editor = "vim"
			}
		}
	}

	editing := c.getOptBool("edit", true)

	tmpFileNameOrig := fmt.Sprintf("%s.orig", tmpFileName)
	util.CopyFile(tmpFileName, tmpFileNameOrig)
	defer func() {
		os.Remove(tmpFileNameOrig)
	}()

	for true {
		if editing {
			shell, _ := shellquote.Split(editor)
			shell = append(shell, tmpFileName)
			log.Debug("Running: %#v", shell)
			cmd := exec.Command(shell[0], shell[1:]...)
			cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
			if err := cmd.Run(); err != nil {
				log.Error("Failed to edit template with %s: %s", editor, err)
				if util.PromptYN("edit again?", true) {
					continue
				}
				return err
			}

			diff := exec.Command("diff", "-q", tmpFileNameOrig, tmpFileName)
			// if err == nil then diff found no changes
			if err := diff.Run(); err == nil {
				return NoChangesFound{}
			}
		}

		edited := make(map[string]interface{})
		if fh, err := ioutil.ReadFile(tmpFileName); err != nil {
			log.Error("Failed to read tmpfile %s: %s", tmpFileName, err)
			if editing && util.PromptYN("edit again?", true) {
				continue
			}
			return err
		} else {
			if err := yaml.Unmarshal(fh, &edited); err != nil {
				log.Error("Failed to parse YAML: %s", err)
				if editing && util.PromptYN("edit again?", true) {
					continue
				}
				return err
			}
		}

		if fixed, err := util.YamlFixup(edited); err != nil {
			return err
		} else {
			edited = fixed.(map[string]interface{})
		}

		// if you want to abort editing a jira issue then
		// you can add the "abort: true" flag to the document
		// and we will abort now
		if val, ok := edited["abort"].(bool); ok && val {
			log.Info("abort flag found in template, quiting")
			return fmt.Errorf("abort flag found in template, quiting")
		}

		if _, ok := templateData["meta"]; ok {
			mf := templateData["meta"].(map[string]interface{})["fields"]
			if f, ok := edited["fields"].(map[string]interface{}); ok {
				for k := range f {
					if _, ok := mf.(map[string]interface{})[k]; !ok {
						err := fmt.Errorf("Field %s is not editable", k)
						log.Error("%s", err)
						if editing && util.PromptYN("edit again?", true) {
							continue
						}
						return err
					}
				}
			}
		}

		json, err := util.JsonEncode(edited)
		if err != nil {
			return err
		}

		if err := templateProcessor(json); err != nil {
			log.Error("%s", err)
			if editing && util.PromptYN("edit again?", true) {
				continue
			}
		}
		return nil
	}
	return nil
}

func (c *Cli) Browse(uri string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", uri).Run()
	} else if runtime.GOOS == "linux" {
		return exec.Command("xdg-open", uri).Run()
	}
	return nil
}

func (c *Cli) getOptString(optName string, dflt string) string {
	if val, ok := c.Opts[optName].(string); ok {
		return val
	} else {
		return dflt
	}
}

func (c *Cli) getOptBool(optName string, dflt bool) bool {
	if val, ok := c.Opts[optName].(bool); ok {
		return val
	} else {
		return dflt
	}
}

func (c *Cli) Login() error {
	return fmt.Errorf("Login not implemented")
}

func (c *Cli) ExportTemplates() error {
	dir := c.Opts["directory"].(string)
	if err := util.Mkdir(dir); err != nil {
		return err
	}

	for name, template := range c.Templates {
		if wanted, ok := c.Opts["template"]; ok && wanted != name {
			continue
		}
		templateFile := fmt.Sprintf("%s/%s", dir, name)
		if _, err := os.Stat(templateFile); err == nil {
			log.Warning("Skipping %s, already exists", templateFile)
			continue
		}
		if fh, err := os.OpenFile(templateFile, os.O_WRONLY|os.O_CREATE, 0644); err != nil {
			log.Error("Failed to open %s for writing: %s", templateFile, err)
			return err
		} else {
			defer fh.Close()
			log.Notice("Creating %s", templateFile)
			fh.Write([]byte(template))
		}
	}
	return nil
}
