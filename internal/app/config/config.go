package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Config struct {
	App           App                      `json:"app"`
	Duration      time.Duration            `json:"duration"`       // интервал удаления по умолчанию
	BotExceptions map[string]time.Duration `json:"bot_exceptions"` // ключ - юзернейм бота, значение - интервал удаления
	AdminUsers    map[int64]struct{}       `json:"admin_users"`
}

type App struct {
	LoggerLevel string `json:"logger_level"`
	TelegramAPI string `json:"telegram_api"`
}

type Manager struct {
	mu   sync.RWMutex
	cfg  *Config
	path string
}

func New(path string) (*Manager, error) {
	m := &Manager{path: path}

	var err error
	m.cfg, err = m.readParseValidate(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			m.cfg = m.GetDefault()
			data, err := json.MarshalIndent(m.cfg, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("marshal config: %w", err)
			}
			if err := m.writeAtomic(path, data, 0644); err != nil {
				return nil, fmt.Errorf("write config: %w", err)
			}
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	return m, nil
}

func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.cfg
}

func (m *Manager) Update(modify func(cfg *Config)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cfg == nil {
		return errors.New("no config loaded")
	}

	modify(m.cfg)

	if err := m.validate(m.cfg); err != nil {
		return fmt.Errorf("invalid config update: %w", err)
	}

	return m.saveLocked()
}

func (m *Manager) GetDefault() *Config {
	return &Config{
		App:           App{},
		Duration:      time.Minute,
		BotExceptions: make(map[string]time.Duration),
		AdminUsers:    make(map[int64]struct{}),
	}
}

func (m *Manager) readParseValidate(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open/read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	if err := m.validate(&cfg); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}
	return &cfg, nil
}

func (m *Manager) validate(cfg *Config) error {
	// app
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if cfg.App.LoggerLevel != "" && !validLevels[cfg.App.LoggerLevel] {
		return fmt.Errorf("app.logger_level must be one of debug, info, warn, error; got %s", cfg.App.LoggerLevel)
	}

	if cfg.App.TelegramAPI == "" {
		return errors.New("app.telegram_api is required")
	}

	if cfg.Duration == 0 {
		cfg.Duration = time.Minute
	}

	if cfg.BotExceptions == nil {
		cfg.BotExceptions = make(map[string]time.Duration)
	}

	if cfg.AdminUsers == nil {
		cfg.AdminUsers = make(map[int64]struct{})
	}

	return nil
}

func (m *Manager) saveLocked() error {
	if m.path == "" {
		return errors.New("no config file loaded")
	}
	if m.cfg == nil {
		return errors.New("no config to save")
	}

	data, err := json.MarshalIndent(m.cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return m.writeAtomic(m.path, data, 0644)
}

func (m *Manager) writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp := filepath.Join(dir, fmt.Sprintf(".%s.tmp-%d", base, time.Now().UnixNano()))

	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
