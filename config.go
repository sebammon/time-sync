package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type config struct {
	ClockifyAPIKey      string `json:"clockify_api_key"`
	ClockifyWorkspaceID string `json:"clockify_workspace_id"`
	ClockifyUserID      string `json:"clockify_user_id"`
	YouTrackAPIKey      string `json:"youtrack_api_key"`
}

var configKeys = []string{
	"clockify-api-key",
	"clockify-workspace-id",
	"clockify-user-id",
	"youtrack-api-key",
}

func configPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "time-sync", "config.json"), nil
}

func loadConfig() (*config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &config{}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &cfg, nil
}

func saveConfig(cfg *config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (c *config) validate() error {
	var missing []string
	if c.ClockifyAPIKey == "" {
		missing = append(missing, "clockify-api-key")
	}
	if c.ClockifyWorkspaceID == "" {
		missing = append(missing, "clockify-workspace-id")
	}
	if c.ClockifyUserID == "" {
		missing = append(missing, "clockify-user-id")
	}
	if c.YouTrackAPIKey == "" {
		missing = append(missing, "youtrack-api-key")
	}
	if len(missing) == 0 {
		return nil
	}
	path, _ := configPath()
	return fmt.Errorf("missing config values: %v\nset them with: time-sync config set <key> <value>\nconfig file: %s", missing, path)
}

func (c *config) field(key string) (*string, bool) {
	switch key {
	case "clockify-api-key":
		return &c.ClockifyAPIKey, true
	case "clockify-workspace-id":
		return &c.ClockifyWorkspaceID, true
	case "clockify-user-id":
		return &c.ClockifyUserID, true
	case "youtrack-api-key":
		return &c.YouTrackAPIKey, true
	}
	return nil, false
}

func runConfigCmd(args []string) error {
	if len(args) == 0 {
		return configUsageErr()
	}

	switch args[0] {
	case "set":
		if len(args) != 3 {
			return fmt.Errorf("usage: time-sync config set <key> <value>")
		}
		return configSet(args[1], args[2])
	case "get":
		if len(args) != 2 {
			return fmt.Errorf("usage: time-sync config get <key>")
		}
		return configGet(args[1])
	case "list":
		return configList()
	default:
		return configUsageErr()
	}
}

func configUsageErr() error {
	return fmt.Errorf("usage: time-sync config <set|get|list> [args]\n\nkeys:\n  %s", joinKeys())
}

func joinKeys() string {
	return fmt.Sprintf("%s", configKeys)
}

func configSet(key, value string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ptr, ok := cfg.field(key)
	if !ok {
		return fmt.Errorf("unknown key %q (valid: %v)", key, configKeys)
	}
	*ptr = value
	if err := saveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("✅ Set %s\n", key)
	return nil
}

func configGet(key string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ptr, ok := cfg.field(key)
	if !ok {
		return fmt.Errorf("unknown key %q (valid: %v)", key, configKeys)
	}
	if *ptr == "" {
		return fmt.Errorf("%s is not set", key)
	}
	fmt.Println(*ptr)
	return nil
}

func configList() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	path, _ := configPath()
	fmt.Printf("config: %s\n\n", path)

	keys := make([]string, len(configKeys))
	copy(keys, configKeys)
	sort.Strings(keys)

	for _, k := range keys {
		ptr, _ := cfg.field(k)
		fmt.Printf("  %-25s %s\n", k, maskSecret(*ptr))
	}
	return nil
}

func maskSecret(v string) string {
	if v == "" {
		return "(unset)"
	}
	if len(v) <= 4 {
		return "****"
	}
	return "****" + v[len(v)-4:]
}
