package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		HTTPAddr   string   `yaml:"http_addr"`
		HTTPSAddr  string   `yaml:"https_addr"`
		APIAddr    string   `yaml:"api_addr"`
		AdminHosts []string `yaml:"admin_hosts"`
		HTTP3      *bool    `yaml:"http3"`
	} `yaml:"server"`
	Database struct {
		URL string `yaml:"url"`
	} `yaml:"database"`
	Auth struct {
		JWTSecret     string `yaml:"jwt_secret"`
		SessionSecret string `yaml:"session_secret"`
	} `yaml:"auth"`
	Certs struct {
		StorageDir string `yaml:"storage_dir"`
	} `yaml:"certs"`
	GeoIP struct {
		DBPath string `yaml:"db_path"`
	} `yaml:"geoip"`
	CrowdSec struct {
		Enabled        bool   `yaml:"enabled"`
		LAPIURL        string `yaml:"lapi_url"`
		APIKey         string `yaml:"api_key"`
		RefreshSeconds int    `yaml:"refresh_seconds"`
	} `yaml:"crowdsec"`
	Logging struct {
		Dir           string `yaml:"dir"`
		MaxMemory     int    `yaml:"max_memory"`
		RetentionDays int    `yaml:"retention_days"`
	} `yaml:"logging"`
	Cache struct {
		Enabled     *bool `yaml:"enabled"`
		TTLSeconds  int   `yaml:"ttl_seconds"`
		MaxEntries  int   `yaml:"max_entries"`
		MaxObjectKB int   `yaml:"max_object_kb"`
		MaxTotalMB  int   `yaml:"max_total_mb"`
	} `yaml:"cache"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	// defaults
	if c.Server.HTTPAddr == "" {
		c.Server.HTTPAddr = ":80"
	}
	if c.Server.HTTPSAddr == "" {
		c.Server.HTTPSAddr = ":443"
	}
	if c.Server.APIAddr == "" {
		c.Server.APIAddr = ":8080"
	}
	if c.Logging.Dir == "" {
		c.Logging.Dir = "/var/lib/bastion/logs"
	}
	if c.Logging.MaxMemory == 0 {
		c.Logging.MaxMemory = 10000
	}
	if c.Logging.RetentionDays == 0 {
		c.Logging.RetentionDays = 30
	}
	return &c, nil
}

func (c *Config) Validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("database.url required")
	}
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("auth.jwt_secret required")
	}
	return nil
}
