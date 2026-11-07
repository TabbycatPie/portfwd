package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Item struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	TargetHost string `json:"target_host"`
	TargetPort int    `json:"target_port"`
	LocalPort  int    `json:"local_port"`
	Enabled    bool   `json:"enabled"`
}

type Config struct {
	Items []Item `json:"items"`
}

func configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = os.Getenv("APPDATA")
	}
	d := filepath.Join(dir, "PortFwd")
	_ = os.MkdirAll(d, 0755)
	return filepath.Join(d, "config.json")
}

// stripBOM 去掉 UTF-8 BOM：记事本另存为「UTF-8」会带 BOM，直接 Unmarshal 会失败
func stripBOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

func loadConfig() Config {
	var c Config
	b, err := os.ReadFile(configPath())
	if err != nil {
		return Config{}
	}
	_ = json.Unmarshal(stripBOM(b), &c)
	// 补齐 ID
	need := false
	for i := range c.Items {
		if c.Items[i].ID == "" {
			c.Items[i].ID = newID()
			need = true
		}
	}
	if need {
		saveConfig(&c)
	}
	return c
}

func saveConfig(c *Config) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), b, 0644)
}

var idSeq = 0

func newID() string {
	idSeq++
	return fmt.Sprintf("f%d-%d", nowUnix(), idSeq)
}

func (it Item) Target() string {
	return it.TargetHost + ":" + strconv.Itoa(it.TargetPort)
}

func (it Item) Local() string {
	return ":" + strconv.Itoa(it.LocalPort)
}
