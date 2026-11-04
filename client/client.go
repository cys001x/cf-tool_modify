package client

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gitcode.com/sheng_wang/cf-tool_modify/cookiejar"
	"github.com/fatih/color"
)

// Client codeforces client
type Client struct {
	Jar            *cookiejar.Jar `json:"cookies"`
	Handle         string         `json:"handle"`
	HandleOrEmail  string         `json:"handle_or_email"`
	Password       string         `json:"password"`
	Ftaa           string         `json:"ftaa"`
	Bfaa           string         `json:"bfaa"`
	LastSubmission *Info          `json:"last_submission"`
	LastVerdict    string         `json:"-"`
	host           string
	proxy          string
	path           string
	client         *http.Client
}

// Instance global client
var Instance *Client

const defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// configUA is the User-Agent stored in the user's configuration file.
var configUA string

// SetUserAgent records the User-Agent taken from the configuration file.
// An empty value falls back to defaultUA.
func SetUserAgent(ua string) {
	configUA = strings.TrimSpace(ua)
}

// GetUserAgent returns the User-Agent currently taken from the configuration
// (not the CF_UA environment override). Used to persist a value learned at
// login time.
func GetUserAgent() string {
	return configUA
}

// browserUA returns the User-Agent sent with every request.
// Cloudflare binds cf_clearance to the User-Agent that obtained it, so this
// must match the browser the cookies were exported from.
//
// Priority: CF_UA environment variable, then the configured value, then the
// built-in default.
func browserUA() string {
	if ua := strings.TrimSpace(os.Getenv("CF_UA")); ua != "" {
		return ua
	}
	if configUA != "" {
		return configUA
	}
	return defaultUA
}

// uaTransport injects a browser User-Agent so Cloudflare accepts the cf_clearance cookie.
type uaTransport struct {
	inner http.RoundTripper
}

func (t *uaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", browserUA())
	return t.inner.RoundTrip(req)
}

// Init initialize
func Init(path, host, proxy string) {
	jar, _ := cookiejar.New(nil)
	c := &Client{Jar: jar, LastSubmission: nil, path: path, host: host, proxy: proxy, client: nil}
	if err := c.load(); err != nil {
		color.Red(err.Error())
		color.Green("Create a new session in %v", path)
	}
	Proxy := http.ProxyFromEnvironment
	if len(proxy) > 0 {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			color.Red(err.Error())
			color.Green("Use default proxy from environment")
		} else {
			Proxy = http.ProxyURL(proxyURL)
		}
	}
	c.client = &http.Client{Jar: c.Jar, Transport: &uaTransport{inner: &http.Transport{Proxy: Proxy}}}
	if err := c.save(); err != nil {
		color.Red(err.Error())
	}
	Instance = c
}

// load from path
func (c *Client) load() (err error) {
	file, err := os.Open(c.path)
	if err != nil {
		return
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)

	if err != nil {
		return err
	}

	return json.Unmarshal(bytes, c)
}

// save file to path
func (c *Client) save() (err error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err == nil {
		os.MkdirAll(filepath.Dir(c.path), os.ModePerm)
		err = ioutil.WriteFile(c.path, data, 0644)
	}
	if err != nil {
		color.Red("Cannot save session to %v\n%v", c.path, err.Error())
	}
	return
}
