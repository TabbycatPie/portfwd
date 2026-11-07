package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
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
	// UIScale 界面缩放（1.0 = 100%），叠加在系统 DPI 之上
	UIScale float64 `json:"ui_scale,omitempty"`
	// Lang 界面语言："zh" / "en"；留空 = 跟随系统
	Lang string `json:"lang,omitempty"`
}

var (
	cfgPathOnce sync.Once
	cfgPathVal  string
)

// 配置路径：装了服务就用 %ProgramData%\PortFwd\config.json（服务和面板共用一份），
// 否则用当前用户的 %APPDATA%\PortFwd\config.json。
func configPath() string {
	cfgPathOnce.Do(func() {
		dir, err := os.UserConfigDir()
		if err != nil || dir == "" {
			dir = os.Getenv("APPDATA")
		}
		userPath := filepath.Join(dir, "PortFwd", "config.json")
		if serviceInstalled() {
			pd := filepath.Join(os.Getenv("ProgramData"), "PortFwd")
			_ = os.MkdirAll(pd, 0755)
			svcPath := filepath.Join(pd, "config.json")
			if _, err := os.Stat(svcPath); err != nil {
				// 首次切到服务模式：把用户侧配置迁过去
				if b, e := os.ReadFile(userPath); e == nil {
					_ = os.WriteFile(svcPath, b, 0644)
				}
			}
			cfgPathVal = svcPath
			return
		}
		_ = os.MkdirAll(filepath.Dir(userPath), 0755)
		cfgPathVal = userPath
	})
	return cfgPathVal
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
