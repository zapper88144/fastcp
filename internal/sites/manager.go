package sites

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/fastcp/fastcp/internal/config"
	"github.com/fastcp/fastcp/internal/models"
)

var (
	ErrSiteNotFound    = errors.New("site not found")
	ErrDomainExists    = errors.New("domain already exists")
	ErrInvalidPHPVersion = errors.New("invalid PHP version")
)

// Manager manages website configurations
type Manager struct {
	sites    map[string]*models.Site
	domains  map[string]string // domain -> site ID
	mu       sync.RWMutex
	dataPath string
}

// NewManager creates a new site manager
func NewManager(dataPath string) *Manager {
	return &Manager{
		sites:    make(map[string]*models.Site),
		domains:  make(map[string]string),
		dataPath: dataPath,
	}
}

// Load loads sites from storage
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sitesFile := filepath.Join(m.dataPath, "sites.json")
	data, err := os.ReadFile(sitesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No sites yet
		}
		return err
	}

	var sites []*models.Site
	if err := json.Unmarshal(data, &sites); err != nil {
		return err
	}

	for _, site := range sites {
		m.sites[site.ID] = site
		m.domains[site.Domain] = site.ID
		for _, alias := range site.Aliases {
			m.domains[alias] = site.ID
		}
	}

	return nil
}

// Save saves sites to storage
func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sites := make([]*models.Site, 0, len(m.sites))
	for _, site := range m.sites {
		sites = append(sites, site)
	}

	data, err := json.MarshalIndent(sites, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(m.dataPath, 0755); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(m.dataPath, "sites.json"), data, 0644)
}

// Create creates a new site
func (m *Manager) Create(site *models.Site) (*models.Site, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if domain already exists
	if _, exists := m.domains[site.Domain]; exists {
		return nil, ErrDomainExists
	}
	for _, alias := range site.Aliases {
		if _, exists := m.domains[alias]; exists {
			return nil, fmt.Errorf("alias %s already exists", alias)
		}
	}

	// Validate PHP version
	cfg := config.Get()
	validPHP := false
	for _, pv := range cfg.PHPVersions {
		if pv.Version == site.PHPVersion && pv.Enabled {
			validPHP = true
			break
		}
	}
	if !validPHP {
		return nil, ErrInvalidPHPVersion
	}

	// Set defaults
	if site.ID == "" {
		site.ID = uuid.New().String()
	}
	if site.Status == "" {
		site.Status = "active"
	}
	if site.PublicPath == "" {
		site.PublicPath = "public"
	}
	if site.RootPath == "" {
		site.RootPath = filepath.Join(cfg.SitesDir, site.Domain)
	}

	site.CreatedAt = time.Now()
	site.UpdatedAt = time.Now()

	// Create directory structure
	if err := m.createSiteDirectories(site); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	// Add to maps
	m.sites[site.ID] = site
	m.domains[site.Domain] = site.ID
	for _, alias := range site.Aliases {
		m.domains[alias] = site.ID
	}

	// Save to disk
	if err := m.saveUnlocked(); err != nil {
		return nil, err
	}

	return site, nil
}

// Get retrieves a site by ID
func (m *Manager) Get(id string) (*models.Site, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	site, ok := m.sites[id]
	if !ok {
		return nil, ErrSiteNotFound
	}

	return site, nil
}

// GetByDomain retrieves a site by domain
func (m *Manager) GetByDomain(domain string) (*models.Site, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.domains[domain]
	if !ok {
		return nil, ErrSiteNotFound
	}

	return m.sites[id], nil
}

// List returns all sites, optionally filtered by user
func (m *Manager) List(userID string) []*models.Site {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sites := make([]*models.Site, 0, len(m.sites))
	for _, site := range m.sites {
		if userID == "" || site.UserID == userID {
			sites = append(sites, site)
		}
	}

	return sites
}

// GetAll returns all sites as a slice
func (m *Manager) GetAll() []models.Site {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sites := make([]models.Site, 0, len(m.sites))
	for _, site := range m.sites {
		sites = append(sites, *site)
	}

	return sites
}

// Update updates an existing site
func (m *Manager) Update(id string, updates *models.Site) (*models.Site, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	site, ok := m.sites[id]
	if !ok {
		return nil, ErrSiteNotFound
	}

	// Check if new domain conflicts with existing
	if updates.Domain != "" && updates.Domain != site.Domain {
		if _, exists := m.domains[updates.Domain]; exists {
			return nil, ErrDomainExists
		}
		// Remove old domain mapping
		delete(m.domains, site.Domain)
		site.Domain = updates.Domain
		m.domains[updates.Domain] = id
	}

	// Update fields
	if updates.Name != "" {
		site.Name = updates.Name
	}
	if updates.PHPVersion != "" {
		// Validate PHP version
		cfg := config.Get()
		validPHP := false
		for _, pv := range cfg.PHPVersions {
			if pv.Version == updates.PHPVersion && pv.Enabled {
				validPHP = true
				break
			}
		}
		if !validPHP {
			return nil, ErrInvalidPHPVersion
		}
		site.PHPVersion = updates.PHPVersion
	}
	if updates.Aliases != nil {
		// Remove old alias mappings
		for _, alias := range site.Aliases {
			delete(m.domains, alias)
		}
		// Add new alias mappings
		for _, alias := range updates.Aliases {
			if existingID, exists := m.domains[alias]; exists && existingID != id {
				return nil, fmt.Errorf("alias %s already exists", alias)
			}
			m.domains[alias] = id
		}
		site.Aliases = updates.Aliases
	}
	if updates.PublicPath != "" {
		site.PublicPath = updates.PublicPath
	}
	if updates.Status != "" {
		site.Status = updates.Status
	}

	// Worker mode settings
	site.WorkerMode = updates.WorkerMode
	if updates.WorkerFile != "" {
		site.WorkerFile = updates.WorkerFile
	}
	if updates.WorkerNum > 0 {
		site.WorkerNum = updates.WorkerNum
	}

	if updates.Environment != nil {
		site.Environment = updates.Environment
	}

	site.UpdatedAt = time.Now()

	// Save to disk
	if err := m.saveUnlocked(); err != nil {
		return nil, err
	}

	return site, nil
}

// Delete removes a site
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	site, ok := m.sites[id]
	if !ok {
		return ErrSiteNotFound
	}

	// Remove domain mappings
	delete(m.domains, site.Domain)
	for _, alias := range site.Aliases {
		delete(m.domains, alias)
	}

	// Remove site
	delete(m.sites, id)

	// Save to disk
	return m.saveUnlocked()
}

// Suspend suspends a site
func (m *Manager) Suspend(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	site, ok := m.sites[id]
	if !ok {
		return ErrSiteNotFound
	}

	site.Status = "suspended"
	site.UpdatedAt = time.Now()

	return m.saveUnlocked()
}

// Unsuspend reactivates a suspended site
func (m *Manager) Unsuspend(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	site, ok := m.sites[id]
	if !ok {
		return ErrSiteNotFound
	}

	site.Status = "active"
	site.UpdatedAt = time.Now()

	return m.saveUnlocked()
}

// createSiteDirectories creates the directory structure for a site
func (m *Manager) createSiteDirectories(site *models.Site) error {
	cfg := config.Get()

	dirs := []string{
		site.RootPath,
		filepath.Join(site.RootPath, site.PublicPath),
		filepath.Join(cfg.LogDir, "sites", site.ID),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Create default index.php
	indexPath := filepath.Join(site.RootPath, site.PublicPath, "index.php")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		content := fmt.Sprintf(`<?php
// Site: %s
// Domain: %s
// Created by FastCP

echo '<h1>Welcome to %s</h1>';
echo '<p>Your site is ready! Replace this file with your application.</p>';
echo '<p>PHP Version: ' . PHP_VERSION . '</p>';
echo '<p>Server: ' . $_SERVER['SERVER_SOFTWARE'] . '</p>';
`, site.Name, site.Domain, site.Name)
		if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// saveUnlocked saves sites without acquiring lock (caller must hold lock)
func (m *Manager) saveUnlocked() error {
	sites := make([]*models.Site, 0, len(m.sites))
	for _, site := range m.sites {
		sites = append(sites, site)
	}

	data, err := json.MarshalIndent(sites, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(m.dataPath, 0755); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(m.dataPath, "sites.json"), data, 0644)
}

// CountByPHPVersion returns the count of sites using each PHP version
func (m *Manager) CountByPHPVersion() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[string]int)
	for _, site := range m.sites {
		if site.Status == "active" {
			counts[site.PHPVersion]++
		}
	}

	return counts
}

// GetStats returns site statistics
func (m *Manager) GetStats() (total, active int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total = len(m.sites)
	for _, site := range m.sites {
		if site.Status == "active" {
			active++
		}
	}

	return
}

