package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/fastcp/fastcp/internal/limits"
	"github.com/fastcp/fastcp/internal/middleware"
	"github.com/fastcp/fastcp/internal/models"
)

// FastCPUser represents a FastCP user with limits and usage
type FastCPUser struct {
	Username      string `json:"username"`
	UID           int    `json:"uid"`
	GID           int    `json:"gid"`
	HomeDir       string `json:"home_dir"`
	IsAdmin       bool   `json:"is_admin"`
	Enabled       bool   `json:"enabled"`

	// Limits
	SiteLimit     int   `json:"site_limit"`      // 0 = unlimited
	DiskLimitMB   int64 `json:"disk_limit_mb"`   // 0 = unlimited
	RAMLimitMB    int64 `json:"ram_limit_mb"`    // 0 = unlimited
	CPUPercent    int   `json:"cpu_percent"`     // 0 = unlimited (100 = 1 core)
	MaxProcesses  int   `json:"max_processes"`   // 0 = unlimited

	// Usage
	SiteCount     int   `json:"site_count"`
	DiskUsedMB    int64 `json:"disk_used_mb"`
	RAMUsedMB     int64 `json:"ram_used_mb"`
	ProcessCount  int   `json:"process_count"`
}

// CreateUserRequest represents a request to create a user
type CreateUserRequest struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	IsAdmin      bool   `json:"is_admin"`      // Add to sudo group

	// Resource limits
	SiteLimit    int   `json:"site_limit"`     // 0 = unlimited
	DiskLimitMB  int64 `json:"disk_limit_mb"`  // 0 = unlimited
	RAMLimitMB   int64 `json:"ram_limit_mb"`   // 0 = unlimited
	CPUPercent   int   `json:"cpu_percent"`    // 0 = unlimited
	MaxProcesses int   `json:"max_processes"`  // 0 = unlimited
}

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	Password     string `json:"password,omitempty"`
	Enabled      bool   `json:"enabled"`

	// Resource limits
	SiteLimit    int   `json:"site_limit"`
	DiskLimitMB  int64 `json:"disk_limit_mb"`
	RAMLimitMB   int64 `json:"ram_limit_mb"`
	CPUPercent   int   `json:"cpu_percent"`
	MaxProcesses int   `json:"max_processes"`
}

// listUsers returns all FastCP users
func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.getFastCPUsers()
	if err != nil {
		s.error(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	s.success(w, map[string]interface{}{
		"users": users,
		"total": len(users),
	})
}

// getUser returns a single user
func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	u, err := s.getFastCPUser(username)
	if err != nil {
		s.error(w, http.StatusNotFound, "user not found")
		return
	}

	s.success(w, u)
}

// createUser creates a new Unix user
func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)

	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate
	if req.Username == "" {
		s.error(w, http.StatusBadRequest, "username is required")
		return
	}
	if req.Password == "" {
		s.error(w, http.StatusBadRequest, "password is required")
		return
	}
	if len(req.Password) < 8 {
		s.error(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	// Check if user already exists
	if _, err := user.Lookup(req.Username); err == nil {
		s.error(w, http.StatusConflict, "user already exists")
		return
	}

	// Create user with useradd
	cmd := exec.Command("useradd", "-m", "-s", "/bin/bash", req.Username)
	if output, err := cmd.CombinedOutput(); err != nil {
		s.logger.Error("failed to create user", "error", err, "output", string(output))
		s.error(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Set password
	cmd = exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("%s:%s", req.Username, req.Password))
	if output, err := cmd.CombinedOutput(); err != nil {
		s.logger.Error("failed to set password", "error", err, "output", string(output))
		// Cleanup: delete the user
		_ = exec.Command("userdel", "-r", req.Username).Run()
		s.error(w, http.StatusInternalServerError, "failed to set password")
		return
	}

	// Add to fastcp group
	_ = exec.Command("groupadd", "-f", "fastcp").Run()
	cmd = exec.Command("usermod", "-aG", "fastcp", req.Username)
	if output, err := cmd.CombinedOutput(); err != nil {
		s.logger.Warn("failed to add user to fastcp group", "error", err, "output", string(output))
	}

	// Add to sudo group if admin
	if req.IsAdmin {
		cmd = exec.Command("usermod", "-aG", "sudo", req.Username)
		if output, err := cmd.CombinedOutput(); err != nil {
			s.logger.Warn("failed to add user to sudo group", "error", err, "output", string(output))
		}
	}

	// Set resource limits
	userLimits := &models.UserLimits{
		Username:      req.Username,
		MaxSites:      req.SiteLimit,
		MaxDiskMB:     req.DiskLimitMB,
		MaxRAMMB:      req.RAMLimitMB,
		MaxCPUPercent: req.CPUPercent,
		MaxProcesses:  req.MaxProcesses,
	}

	if err := s.siteManager.SetUserLimit(userLimits); err != nil {
		s.logger.Warn("failed to save user limits", "error", err)
	}

	// Apply system-level limits (cgroups, quotas)
	limitsManager := limits.NewManager(s.logger)
	if err := limitsManager.ApplyLimits(userLimits); err != nil {
		s.logger.Warn("failed to apply system limits", "error", err)
	}

	s.logger.Info("user created", "username", req.Username, "by", claims.Username)

	// Return the created user
	u, _ := s.getFastCPUser(req.Username)
	s.json(w, http.StatusCreated, u)
}

// updateUser updates a user's settings
func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	claims := middleware.GetClaims(r)

	// Verify user exists
	if _, err := user.Lookup(username); err != nil {
		s.error(w, http.StatusNotFound, "user not found")
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Update password if provided
	if req.Password != "" {
		if len(req.Password) < 8 {
			s.error(w, http.StatusBadRequest, "password must be at least 8 characters")
			return
		}
		cmd := exec.Command("chpasswd")
		cmd.Stdin = strings.NewReader(fmt.Sprintf("%s:%s", username, req.Password))
		if output, err := cmd.CombinedOutput(); err != nil {
			s.logger.Error("failed to set password", "error", err, "output", string(output))
			s.error(w, http.StatusInternalServerError, "failed to update password")
			return
		}
	}

	// Update resource limits
	userLimits := &models.UserLimits{
		Username:      username,
		MaxSites:      req.SiteLimit,
		MaxDiskMB:     req.DiskLimitMB,
		MaxRAMMB:      req.RAMLimitMB,
		MaxCPUPercent: req.CPUPercent,
		MaxProcesses:  req.MaxProcesses,
	}

	if err := s.siteManager.SetUserLimit(userLimits); err != nil {
		s.logger.Warn("failed to save user limits", "error", err)
	}

	// Apply system-level limits
	limitsManager := limits.NewManager(s.logger)
	if err := limitsManager.ApplyLimits(userLimits); err != nil {
		s.logger.Warn("failed to apply system limits", "error", err)
	}

	// Enable/disable user
	if !req.Enabled {
		_ = exec.Command("usermod", "-L", username).Run() // Lock account
	} else {
		_ = exec.Command("usermod", "-U", username).Run() // Unlock account
	}

	s.logger.Info("user updated", "username", username, "by", claims.Username)

	u, _ := s.getFastCPUser(username)
	s.success(w, u)
}

// deleteUser removes a Unix user
func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	claims := middleware.GetClaims(r)

	// Prevent deleting self
	if username == claims.Username {
		s.error(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	// Prevent deleting root
	if username == "root" {
		s.error(w, http.StatusBadRequest, "cannot delete root user")
		return
	}

	// Check if user has sites
	sites := s.siteManager.List(username)
	if len(sites) > 0 {
		s.error(w, http.StatusBadRequest, fmt.Sprintf("user has %d sites, delete them first", len(sites)))
		return
	}

	// Delete user
	cmd := exec.Command("userdel", "-r", username)
	if output, err := cmd.CombinedOutput(); err != nil {
		s.logger.Error("failed to delete user", "error", err, "output", string(output))
		s.error(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	s.logger.Info("user deleted", "username", username, "by", claims.Username)
	s.success(w, map[string]string{"message": "user deleted"})
}

// getFastCPUsers returns all users in the fastcp group
func (s *Server) getFastCPUsers() ([]FastCPUser, error) {
	var users []FastCPUser

	// Get users in fastcp group
	cmd := exec.Command("getent", "group", "fastcp")
	output, err := cmd.Output()
	if err != nil {
		// Group doesn't exist yet
		return users, nil
	}

	// Parse group: fastcp:x:1001:user1,user2,user3
	parts := strings.Split(strings.TrimSpace(string(output)), ":")
	if len(parts) < 4 || parts[3] == "" {
		return users, nil
	}

	usernames := strings.Split(parts[3], ",")
	for _, username := range usernames {
		if u, err := s.getFastCPUser(username); err == nil {
			users = append(users, *u)
		}
	}

	// Also add root if not already in list
	if rootUser, err := s.getFastCPUser("root"); err == nil {
		found := false
		for _, u := range users {
			if u.Username == "root" {
				found = true
				break
			}
		}
		if !found {
			users = append([]FastCPUser{*rootUser}, users...)
		}
	}

	return users, nil
}

// getFastCPUser returns info about a single user
func (s *Server) getFastCPUser(username string) (*FastCPUser, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, err
	}

	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	// Check if admin (in sudo/wheel group)
	isAdmin := s.isUserInGroup(username, "sudo") || s.isUserInGroup(username, "wheel") || username == "root"

	// Check if enabled (account not locked)
	enabled := true
	cmd := exec.Command("passwd", "-S", username)
	if output, err := cmd.Output(); err == nil {
		fields := strings.Fields(string(output))
		if len(fields) >= 2 && fields[1] == "L" {
			enabled = false
		}
	}

	// Get site count
	sites := s.siteManager.List(u.Uid)
	siteCount := len(sites)

	// Get resource limits
	userLimits := s.siteManager.GetUserLimit(username)

	// Get resource usage
	limitsManager := limits.NewManager(s.logger)
	usage, _ := limitsManager.GetUsage(username)

	fastcpUser := &FastCPUser{
		Username:     username,
		UID:          uid,
		GID:          gid,
		HomeDir:      u.HomeDir,
		IsAdmin:      isAdmin,
		Enabled:      enabled,

		// Limits
		SiteLimit:    userLimits.MaxSites,
		DiskLimitMB:  userLimits.MaxDiskMB,
		RAMLimitMB:   userLimits.MaxRAMMB,
		CPUPercent:   userLimits.MaxCPUPercent,
		MaxProcesses: userLimits.MaxProcesses,

		// Current usage
		SiteCount:    siteCount,
	}

	// Add usage stats if available
	if usage != nil {
		fastcpUser.DiskUsedMB = usage.DiskUsedMB
		fastcpUser.RAMUsedMB = usage.RAMUsedMB
		fastcpUser.ProcessCount = usage.ProcessCount
	}

	return fastcpUser, nil
}

// isUserInGroup checks if user is in a group
func (s *Server) isUserInGroup(username, groupName string) bool {
	cmd := exec.Command("groups", username)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), groupName)
}


