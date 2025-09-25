package game

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const defaultAdminAccount = "admin"

type accountRecord struct {
	Password    string    `json:"password"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	LastLogin   time.Time `json:"last_login,omitempty"`
	TotalLogins int       `json:"total_logins,omitempty"`
}

// AccountStats summarises persistent account metadata used for in-game displays.
type AccountStats struct {
	CreatedAt   time.Time
	LastLogin   time.Time
	TotalLogins int
}

type AccountManager struct {
	mu           sync.RWMutex
	accounts     map[string]accountRecord
	path         string
	playersPath  string
	adminAccount string
}

func NewAccountManager(path string) (*AccountManager, error) {
	manager := &AccountManager{
		accounts:     make(map[string]accountRecord),
		path:         path,
		playersPath:  filepath.Join(filepath.Dir(path), "players"),
		adminAccount: defaultAdminAccount,
	}
	if err := manager.load(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (a *AccountManager) playerFilePath(name string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(name)))
	filename := hex.EncodeToString(sum[:]) + ".json"
	return filepath.Join(a.playersPath, filename)
}

func (a *AccountManager) loadPlayerProfile(name string) (PlayerProfile, bool) {
	if a.playersPath == "" {
		return PlayerProfile{}, false
	}
	path := a.playerFilePath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return PlayerProfile{}, false
	}
	type playerRecord struct {
		Room     RoomID            `json:"room,omitempty"`
		Home     RoomID            `json:"home,omitempty"`
		Channels map[string]bool   `json:"channels,omitempty"`
		Aliases  map[string]string `json:"aliases,omitempty"`
	}
	var record playerRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return PlayerProfile{}, false
	}
	profile := PlayerProfile{
		Room:     record.Room,
		Home:     record.Home,
		Channels: decodeChannelSettings(record.Channels),
		Aliases:  decodeChannelAliases(record.Aliases),
	}
	return profile, true
}

func (a *AccountManager) savePlayerProfile(name string, profile PlayerProfile) error {
	if a.playersPath == "" {
		return nil
	}
	if err := os.MkdirAll(a.playersPath, 0o755); err != nil {
		return fmt.Errorf("create players directory: %w", err)
	}
	tmp, err := os.CreateTemp(a.playersPath, "player-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp player file: %w", err)
	}
	type playerRecord struct {
		Room     RoomID            `json:"room,omitempty"`
		Home     RoomID            `json:"home,omitempty"`
		Channels map[string]bool   `json:"channels,omitempty"`
		Aliases  map[string]string `json:"aliases,omitempty"`
	}
	record := playerRecord{
		Room:     profile.Room,
		Home:     profile.Home,
		Channels: encodeChannelSettings(profile.Channels),
		Aliases:  encodeChannelAliases(profile.Aliases),
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(record); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("write player file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("close temp player file: %w", err)
	}
	target := a.playerFilePath(name)
	if err := os.Rename(tmp.Name(), target); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("replace player file: %w", err)
	}
	return nil
}

func (a *AccountManager) SetAdminAccount(name string) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		trimmed = defaultAdminAccount
	}
	a.mu.Lock()
	a.adminAccount = trimmed
	a.mu.Unlock()
}

func (a *AccountManager) IsAdmin(name string) bool {
	a.mu.RLock()
	admin := a.adminAccount
	a.mu.RUnlock()
	if admin == "" {
		admin = defaultAdminAccount
	}
	return strings.EqualFold(name, admin)
}

func (a *AccountManager) load() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	data, err := os.ReadFile(a.path)
	if errors.Is(err, os.ErrNotExist) {
		a.accounts = make(map[string]accountRecord)
		return nil
	}
	if err != nil {
		return fmt.Errorf("read accounts file: %w", err)
	}
	if len(data) == 0 {
		a.accounts = make(map[string]accountRecord)
		return nil
	}
	var accounts map[string]accountRecord
	if err := json.Unmarshal(data, &accounts); err != nil {
		return fmt.Errorf("decode accounts file: %w", err)
	}
	if accounts == nil {
		accounts = make(map[string]accountRecord)
	}
	a.accounts = accounts
	return nil
}

func (a *AccountManager) saveLocked() error {
	dir := filepath.Dir(a.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create accounts directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "accounts-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp accounts file: %w", err)
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(a.accounts); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("write accounts file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("close temp accounts file: %w", err)
	}
	if err := os.Rename(tmp.Name(), a.path); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("replace accounts file: %w", err)
	}
	return nil
}

func (a *AccountManager) Exists(name string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.accounts[name]
	return ok
}

func (a *AccountManager) Register(name, pass string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.accounts[name]; ok {
		return fmt.Errorf("account already exists")
	}
	now := time.Now().UTC()
	a.accounts[name] = accountRecord{
		Password:    string(hashed),
		CreatedAt:   now,
		LastLogin:   time.Time{},
		TotalLogins: 0,
	}
	if err := a.saveLocked(); err != nil {
		delete(a.accounts, name)
		return err
	}
	return nil
}

func (a *AccountManager) Authenticate(name, pass string) bool {
	a.mu.RLock()
	record, ok := a.accounts[name]
	a.mu.RUnlock()
	if !ok {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(record.Password), []byte(pass)) == nil
}

// Profile retrieves the persisted state for a player. Defaults are returned for
// unknown accounts.
func (a *AccountManager) Profile(name string) PlayerProfile {
	profile := PlayerProfile{
		Room:     StartRoom,
		Home:     StartRoom,
		Channels: defaultChannelSettings(),
	}
	if disk, found := a.loadPlayerProfile(name); found {
		if disk.Room != "" {
			profile.Room = disk.Room
		}
		if disk.Home != "" {
			profile.Home = disk.Home
		}
		if disk.Channels != nil {
			profile.Channels = disk.Channels
		}
		if disk.Aliases != nil {
			profile.Aliases = disk.Aliases
		}
	}
	return profile
}

// SaveProfile persists the provided state for the named account.
func (a *AccountManager) SaveProfile(name string, profile PlayerProfile) error {
	a.mu.RLock()
	_, ok := a.accounts[name]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("account not found")
	}
	if err := a.savePlayerProfile(name, profile); err != nil {
		return err
	}
	return nil
}

// RecordLogin updates bookkeeping for a successful login.
func (a *AccountManager) RecordLogin(name string, when time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	record, ok := a.accounts[name]
	if !ok {
		return fmt.Errorf("account not found")
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = when.UTC()
	}
	record.LastLogin = when.UTC()
	record.TotalLogins++
	a.accounts[name] = record
	return a.saveLocked()
}

// Stats returns account metadata for display purposes.
func (a *AccountManager) Stats(name string) (AccountStats, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	record, ok := a.accounts[name]
	if !ok {
		return AccountStats{}, false
	}
	return AccountStats{
		CreatedAt:   record.CreatedAt,
		LastLogin:   record.LastLogin,
		TotalLogins: record.TotalLogins,
	}, true
}
