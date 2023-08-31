// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package configs

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"runtime"
	"time"

	"github.com/araddon/dateparse"
	"github.com/halorium/env"
	"github.com/komkom/toml"
)

var (
	version      = "dev"
	buildTimeStr string
	buildTime    time.Time
	startTime    = time.Now().UTC()

	cookieHk []byte
	cookieBk []byte
	csrfKey  []byte
	jwtSk    ed25519.PrivateKey
	jwtPk    ed25519.PublicKey
)

func init() {
	buildTime, _ = dateparse.ParseAny(buildTimeStr)
}

// Because we don't need viper's mess for just storing configuration from
// a source.
type config struct {
	Main      configMain      `json:"main" env:"-"`
	Server    configServer    `json:"server" env:"-"`
	Database  configDB        `json:"database" env:"-"`
	Email     configEmail     `json:"email" env:"-"`
	Extractor configExtractor `json:"extractor" env:"-"`
	Worker    configWorker    `json:"worker" env:"-"`
}

type configMain struct {
	LogLevel      string `json:"log_level" env:"READECK_LOG_LEVEL"`
	DevMode       bool   `json:"dev_mode" env:"READECK_DEV_MODE"`
	SecretKey     string `json:"secret_key" env:"-"`
	DataDirectory string `json:"data_directory" env:"-"`
}

type configServer struct {
	Host               string        `json:"host" env:"READECK_SERVER_HOST"`
	Port               int           `json:"port" env:"READECK_SERVER_PORT"`
	Prefix             string        `json:"prefix" env:"READECK_SERVER_PREFIX"`
	AllowedHosts       []string      `json:"allowed_hosts" env:"READECK_ALLOWED_HOSTS"`
	UseXForwardedHost  bool          `json:"use_x_forwarded_host" env:"-"`
	UseXForwardedProto bool          `json:"use_x_forwarded_proto" env:"-"`
	Session            configSession `json:"session" env:"-"`
}

type configDB struct {
	Source string `json:"source" env:"READECK_DATABASE_SOURCE"`
}

type configSession struct {
	CookieName string `json:"cookie_name" env:"-"`
	MaxAge     int    `json:"max_age" env:"-"` // in minutes
}

type configEmail struct {
	Debug       bool   `json:"debug" env:"-"`
	Host        string `json:"host" env:"-"`
	Port        int    `json:"port" env:"-"`
	Username    string `json:"username" env:"-"`
	Password    string `json:"password" env:"-"`
	Encryption  string `json:"encryption" env:"-"`
	Insecure    bool   `json:"insecure" env:"-"`
	From        string `json:"from" env:"-"`
	FromNoReply string `json:"from_noreply" env:"-"`
}

type configWorker struct {
	DSN         string `json:"dsn" env:"-"`
	NumWorkers  int    `json:"num_workers" env:"-"`
	StartWorker bool   `json:"start_worker" env:"-"`
}

type configExtractor struct {
	NumWorkers int                `json:"workers" env:"-"`
	SiteConfig []configSiteConfig `json:"site_config" env:"-"`
	DeniedIPs  []configIPNet      `json:"denied_ips" env:"-"`
	ProxyMatch []configProxyMatch `json:"proxy_match" env:"-"`
}

type configSiteConfig struct {
	Name string `json:"name" env:"-"`
	Src  string `json:"src" env:"-"`
}

type configIPNet struct {
	*net.IPNet
}

func newConfigIPNet(v string) configIPNet {
	_, r, _ := net.ParseCIDR(v)
	return configIPNet{IPNet: r}
}

// UnmarshalJSON loads a given string containing an ip address or
// a cidr. If it falls back to a single ip address, it gets a
// /32 or /128 netmask.
func (ci *configIPNet) UnmarshalJSON(d []byte) error {
	var s string
	err := json.Unmarshal(d, &s)
	if err != nil {
		return err
	}

	// Try first to parse a cidr value
	_, r, err := net.ParseCIDR(s)
	if err == nil {
		ci.IPNet = r
		return nil
	}

	// If not cidr notation, then that's an ip with /32 or /128
	r = &net.IPNet{IP: net.ParseIP(s)}
	if r.IP.To4() != nil {
		r.Mask = net.CIDRMask(8*net.IPv4len, 8*net.IPv4len)
	} else {
		r.Mask = net.CIDRMask(8*net.IPv6len, 8*net.IPv6len)
	}
	ci.IPNet = r

	return nil
}

type configProxyMatch struct {
	host string
	url  *url.URL
}

func (pm *configProxyMatch) UnmarshalJSON(d []byte) error {
	var s map[string]string
	err := json.Unmarshal(d, &s)
	if err != nil {
		return err
	}

	if _, ok := s["host"]; !ok {
		return fmt.Errorf(`"host" not in %s`, d)
	}
	if _, ok := s["url"]; !ok {
		return fmt.Errorf(`"url" not in %s`, d)
	}

	proxy, err := url.Parse(s["url"])
	if err != nil {
		return fmt.Errorf("error with proxy URL %s in %s", s["url"], d)
	}

	pm.host = s["host"]
	pm.url = proxy

	return nil
}

func (pm configProxyMatch) Host() string {
	return pm.host
}

func (pm configProxyMatch) URL() *url.URL {
	return pm.url
}

// Config holds the configuration data from configuration files
// or flags.
//
// This variable sets some default values that might be overwritten
// by a configuration file.
var Config = config{
	Main: configMain{
		LogLevel:      "info",
		DevMode:       false,
		DataDirectory: "data",
	},
	Server: configServer{
		Host: "127.0.0.1",
		Port: 5000,
		Session: configSession{
			CookieName: "sxid",
			MaxAge:     86400 * 30, // 60 days
		},
	},
	Database: configDB{},
	Email: configEmail{
		Port: 25,
	},
	Worker: configWorker{
		DSN:         "memory://",
		NumWorkers:  runtime.NumCPU(),
		StartWorker: true,
	},
	Extractor: configExtractor{
		NumWorkers: runtime.NumCPU(),
		SiteConfig: []configSiteConfig{},
		DeniedIPs: []configIPNet{
			newConfigIPNet("127.0.0.0/8"),
			newConfigIPNet("::1/128"),
		},
		ProxyMatch: []configProxyMatch{},
	},
}

// LoadConfiguration loads the configuration file.
func LoadConfiguration(configPath string) error {
	if configPath == "" {
		return nil
	}

	fd, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer fd.Close()

	dec := json.NewDecoder(toml.New(fd))
	if err := dec.Decode(&Config); err != nil {
		return err
	}

	// Override configuration from environment variables
	if err = env.Unmarshal(&Config); err != nil {
		return err
	}

	InitConfiguration()
	return nil
}

// InitConfiguration applies some default computed values on the configuration.
func InitConfiguration() {
	if Config.Database.Source == "" {
		Config.Database.Source = fmt.Sprintf("sqlite3:%s/db.sqlite3", Config.Main.DataDirectory)
	}

	if Config.Email.From == "" {
		Config.Email.From = fmt.Sprintf("noreply@%s", Config.Server.Host)
	}
	if Config.Email.FromNoReply == "" {
		Config.Email.FromNoReply = Config.Email.From
	}

	loadKeys(Config.Main.SecretKey)
}

// loadKeys prepares all the keys derivated from the configuration's
// secret key.
func loadKeys(sk string) {
	// Pad the secret key with its own checksum to have a
	// long enough byte list.
	h := sha512.Sum512([]byte(sk))
	seed := append([]byte(sk), h[:]...)

	hashMsg := func(k []byte, m string) []byte {
		mac := hmac.New(sha256.New, k)
		mac.Write([]byte(m))
		return mac.Sum(nil)
	}

	cookieHk = hashMsg(seed, "cookie-hash-key")
	cookieBk = hashMsg(seed, "cookie-block-key")
	csrfKey = hashMsg(seed, "csrf-key")

	jwtSk = ed25519.NewKeyFromSeed(seed[32:64])
	jwtPk = jwtSk.Public().(ed25519.PublicKey)
}

// CookieHashKey returns the key used by session cookies
func CookieHashKey() []byte {
	return cookieHk
}

// CookieBlockKey returns the key used by session cookies
func CookieBlockKey() []byte {
	return cookieBk
}

// CsrfKey returns the key used by CSRF protection
func CsrfKey() []byte {
	return csrfKey
}

// JwtSk returns the private key for JWT handlers
func JwtSk() ed25519.PrivateKey {
	return jwtSk
}

// JwtPk returns the public key for JWT handlers
func JwtPk() ed25519.PublicKey {
	return jwtPk
}

// ExtractorDeniedIPs returns the value of Config.Extractor.DeniedIPs
// as a slice of *net.IPNet
func ExtractorDeniedIPs() []*net.IPNet {
	res := make([]*net.IPNet, len(Config.Extractor.DeniedIPs))
	for i, x := range Config.Extractor.DeniedIPs {
		res[i] = x.IPNet
	}
	return res
}

// Version returns the current readeck version
func Version() string {
	return version
}

// BuildTime returns the build time or, if empty, the time
// when the application started
func BuildTime() time.Time {
	if buildTime.IsZero() {
		return startTime
	}
	return buildTime
}
