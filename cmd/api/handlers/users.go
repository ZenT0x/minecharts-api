package handlers

import (
	"net/http"
	"strconv"

	"minecharts/cmd/auth"
	"minecharts/cmd/database"
	"minecharts/cmd/logging"

	"github.com/gin-gonic/gin"
)

// UpdateUserRequest represents a request to update user information.
// All fields are optional to allow partial updates.
type UpdateUserRequest struct {
	Username    *string `json:"username" example:"newusername"`
	Email       *string `json:"email" example:"new@example.com"`
	Password    *string `json:"password" example:"newStrongPassword123"`
	Permissions *int64  `json:"permissions" example:"143"` // Bit flags for permissions
	Active      *bool   `json:"active" example:"true"`
}

// PermissionAction represents a single permission action.
type PermissionAction struct {
	Permission int64  `json:"permission" binding:"required" example:"128"`
	Name       string `json:"name" example:"PermViewServer"` // Optionnel, pour la lisibilité
}

// ModifyPermissionsRequest represents a request to modify user permissions.
type ModifyPermissionsRequest struct {
	Permissions []PermissionAction `json:"permissions" binding:"required"`
}

// ListUsersHandler returns a list of all users (admin only).
//
// @Summary      List all users
// @Description  Returns a list of all users in the system (admin only)
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   map[string]interface{}  "List of users"
// @Failure      401  {object}  map[string]string       "Authentication required"
// @Failure      403  {object}  map[string]string       "Permission denied"
// @Failure      500  {object}  map[string]string       "Server error"
// @Router       /users [get]
func ListUsersHandler(c *gin.Context) {
	// Get current admin user for logging
	adminUser, _ := auth.GetCurrentUser(c)

	logging.Auth.WithFields(
		"admin_user_id", adminUser.ID,
		"username", adminUser.Username,
		"remote_ip", c.ClientIP(),
	).Info("Admin requesting list of all users")

	db := database.GetDB()
	users, err := db.ListUsers(c.Request.Context())
	if err != nil {
		logging.DB.WithFields(
			"admin_user_id", adminUser.ID,
			"error", err.Error(),
		).Error("Failed to list users from database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}

	logging.Auth.WithFields(
		"admin_user_id", adminUser.ID,
		"user_count", len(users),
	).Debug("Successfully retrieved user list")

	// Convert to a safer format without password hashes
	response := make([]gin.H, len(users))
	for i, user := range users {
		response[i] = gin.H{
			"id":          user.ID,
			"username":    user.Username,
			"email":       user.Email,
			"permissions": user.Permissions,
			"active":      user.Active,
			"last_login":  user.LastLogin,
			"created_at":  user.CreatedAt,
			"updated_at":  user.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetUserHandler returns details for a specific user.
//
// @Summary      Get user details
// @Description  Returns detailed information about a specific user
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      integer  true  "User ID"
// @Success      200  {object}  map[string]interface{}  "User details"
// @Failure      400  {object}  map[string]string       "Invalid user ID"
// @Failure      401  {object}  map[string]string       "Authentication required"
// @Failure      403  {object}  map[string]string       "Permission denied"
// @Failure      404  {object}  map[string]string       "User not found"
// @Failure      500  {object}  map[string]string       "Server error"
// @Router       /users/{id} [get]
func GetUserHandler(c *gin.Context) {
	// Get current user
	currentUser, ok := auth.GetCurrentUser(c)
	if !ok {
		logging.API.InvalidRequest.WithFields(
			"path", c.Request.URL.Path,
			"remote_ip", c.ClientIP(),
			"error", "not_authenticated",
		).Warn("Get user details failed: user not authenticated")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Get user ID from URL parameter
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logging.API.InvalidRequest.WithFields(
			"current_user_id", currentUser.ID,
			"username", currentUser.Username,
			"requested_id", idStr,
			"remote_ip", c.ClientIP(),
			"error", "invalid_id_format",
		).Warn("Get user details failed: invalid user ID format")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	logging.Auth.Session.WithFields(
		"current_user_id", currentUser.ID,
		"username", currentUser.Username,
		"requested_user_id", id,
		"remote_ip", c.ClientIP(),
	).Debug("User details requested")

	// Users can only view their own details unless they're an admin
	if !currentUser.IsAdmin() && currentUser.ID != id {
		logging.Auth.Session.WithFields(
			"current_user_id", currentUser.ID,
			"username", currentUser.Username,
			"requested_user_id", id,
			"remote_ip", c.ClientIP(),
			"error", "permission_denied",
		).Warn("Get user details failed: non-admin attempting to access another user's details")
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	// Get user from database
	db := database.GetDB()
	user, err := db.GetUserByID(c.Request.Context(), id)
	if err != nil {
		if err == database.ErrUserNotFound {
			logging.Auth.Session.WithFields(
				"current_user_id", currentUser.ID,
				"username", currentUser.Username,
				"requested_user_id", id,
				"error", "user_not_found",
			).Warn("Get user details failed: user not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		logging.DB.WithFields(
			"current_user_id", currentUser.ID,
			"username", currentUser.Username,
			"requested_user_id", id,
			"error", err.Error(),
		).Error("Get user details failed: database error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	logging.Auth.Session.WithFields(
		"current_user_id", currentUser.ID,
		"username", currentUser.Username,
		"requested_user_id", id,
		"requested_username", user.Username,
	).Debug("User details retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"id":          user.ID,
		"username":    user.Username,
		"email":       user.Email,
		"permissions": user.Permissions,
		"active":      user.Active,
		"last_login":  user.LastLogin,
		"created_at":  user.CreatedAt,
		"updated_at":  user.UpdatedAt,
	})
}

// UpdateUserHandler updates a user's information.
//
// @Summary      Update user
// @Description  Updates information for an existing user
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      integer           true  "User ID"
// @Param        request  body      UpdateUserRequest  true  "User information to update"
// @Success      200      {object}  map[string]interface{}  "Updated user details"
// @Failure      400      {object}  map[string]string       "Invalid request"
// @Failure      401      {object}  map[string]string       "Authentication required"
// @Failure      403      {object}  map[string]string       "Permission denied"
// @Failure      404      {object}  map[string]string       "User not found"
// @Failure      500      {object}  map[string]string       "Server error"
// @Router       /users/{id} [put]
func UpdateUserHandler(c *gin.Context) {
	// Get current user
	currentUser, ok := auth.GetCurrentUser(c)
	if !ok {
		logging.API.InvalidRequest.WithFields(
			"path", c.Request.URL.Path,
			"remote_ip", c.ClientIP(),
			"error", "not_authenticated",
		).Warn("Update user failed: user not authenticated")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Get user ID from URL parameter
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logging.API.InvalidRequest.WithFields(
			"current_user_id", currentUser.ID,
			"username", currentUser.Username,
			"requested_id", idStr,
			"remote_ip", c.ClientIP(),
			"error", "invalid_id_format",
		).Warn("Update user failed: invalid user ID format")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	logging.Auth.WithFields(
		"current_user_id", currentUser.ID,
		"username", currentUser.Username,
		"target_user_id", id,
		"remote_ip", c.ClientIP(),
	).Info("User update requested")

	// Users can only update their own details unless they're an admin
	isAdmin := currentUser.IsAdmin()
	isSelf := currentUser.ID == id

	if !isAdmin && !isSelf {
		logging.Auth.WithFields(
			"current_user_id", currentUser.ID,
			"username", currentUser.Username,
			"target_user_id", id,
			"remote_ip", c.ClientIP(),
			"error", "permission_denied",
		).Warn("Update user failed: non-admin attempting to update another user")
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	// Get user from database
	db := database.GetDB()
	user, err := db.GetUserByID(c.Request.Context(), id)
	if err != nil {
		if err == database.ErrUserNotFound {
			logging.Auth.WithFields(
				"current_user_id", currentUser.ID,
				"username", currentUser.Username,
				"target_user_id", id,
				"error", "user_not_found",
			).Warn("Update user failed: target user not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		logging.DB.WithFields(
			"current_user_id", currentUser.ID,
			"username", currentUser.Username,
			"target_user_id", id,
			"error", err.Error(),
		).Error("Update user failed: database error when retrieving user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Parse update request
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logging.API.InvalidRequest.WithFields(
			"current_user_id", currentUser.ID,
			"username", currentUser.Username,
			"target_user_id", id,
			"error", err.Error(),
		).Warn("Update user failed: invalid request format")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log which fields are being updated
	updateFields := make([]string, 0)

	// Apply updates
	if req.Username != nil {
		updateFields = append(updateFields, "username")
		user.Username = *req.Username
	}

	if req.Email != nil {
		updateFields = append(updateFields, "email")
		user.Email = *req.Email
	}

	if req.Password != nil {
		// Only admins or the user themselves can change passwords
		if !isAdmin && !isSelf {
			logging.Auth.WithFields(
				"current_user_id", currentUser.ID,
				"username", currentUser.Username,
				"target_user_id", id,
				"error", "permission_denied",
			).Warn("Update user failed: attempt to change password without permission")
			c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
			return
		}

		updateFields = append(updateFields, "password")
		passwordHash, err := auth.HashPassword(*req.Password)
		if err != nil {
			logging.Auth.WithFields(
				"current_user_id", currentUser.ID,
				"username", currentUser.Username,
				"target_user_id", id,
				"error", err.Error(),
			).Error("Update user failed: password hashing error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		user.PasswordHash = passwordHash
	}

	if req.Permissions != nil {
		// Only admins can change permissions
		if !isAdmin {
			logging.Auth.WithFields(
				"current_user_id", currentUser.ID,
				"username", currentUser.Username,
				"target_user_id", id,
				"error", "permission_denied",
			).Warn("Update user failed: non-admin attempting to change permissions")
			c.JSON(http.StatusForbidden, gin.H{"error": "Only administrators can modify permissions"})
			return
		}
		updateFields = append(updateFields, "permissions")
		user.Permissions = *req.Permissions
	}

	if req.Active != nil {
		// Only admins can change active status
		if !isAdmin {
			logging.Auth.WithFields(
				"current_user_id", currentUser.ID,
				"username", currentUser.Username,
				"target_user_id", id,
				"error", "permission_denied",
			).Warn("Update user failed: non-admin attempting to change account status")
			c.JSON(http.StatusForbidden, gin.H{"error": "Only administrators can change account status"})
			return
		}
		updateFields = append(updateFields, "active")
		user.Active = *req.Active
	}

	logging.Auth.WithFields(
		"current_user_id", currentUser.ID,
		"username", currentUser.Username,
		"target_user_id", id,
		"target_username", user.Username,
		"updated_fields", updateFields,
	).Debug("Applying user updates")

	// Update user in database
	if err := db.UpdateUser(c.Request.Context(), user); err != nil {
		logging.DB.WithFields(
			"current_user_id", currentUser.ID,
			"username", currentUser.Username,
			"target_user_id", id,
			"error", err.Error(),
		).Error("Update user failed: database error when saving updates")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	logging.Auth.WithFields(
		"current_user_id", currentUser.ID,
		"username", currentUser.Username,
		"target_user_id", id,
		"target_username", user.Username,
		"updated_fields", updateFields,
	).Info("User updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"id":          user.ID,
		"username":    user.Username,
		"email":       user.Email,
		"permissions": user.Permissions,
		"active":      user.Active,
		"last_login":  user.LastLogin,
		"updated_at":  user.UpdatedAt,
	})
}

// DeleteUserHandler deletes a user (admin only).
//
// @Summary      Delete user
// @Description  Deletes a user from the system
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      integer  true  "User ID"
// @Success      200  {object}  map[string]string  "User deleted successfully"
// @Failure      400  {object}  map[string]string  "Invalid user ID"
// @Failure      401  {object}  map[string]string  "Authentication required"
// @Failure      403  {object}  map[string]string  "Permission denied"
// @Failure      404  {object}  map[string]string  "User not found"
// @Failure      500  {object}  map[string]string  "Server error"
// @Router       /users/{id} [delete]
func DeleteUserHandler(c *gin.Context) {
	// Get current admin user for logging
	adminUser, _ := auth.GetCurrentUser(c)

	// Get user ID from URL parameter
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logging.API.InvalidRequest.WithFields(
			"admin_user_id", adminUser.ID,
			"user_id_param", idStr,
			"error", "invalid_id_format",
		).Warn("Invalid user ID format in delete request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	logging.Auth.WithFields(
		"admin_user_id", adminUser.ID,
		"username", adminUser.Username,
		"target_user_id", id,
		"remote_ip", c.ClientIP(),
	).Info("Admin attempting to delete user")

	// Don't allow admins to delete themselves
	if adminUser.ID == id {
		logging.Auth.WithFields(
			"admin_user_id", adminUser.ID,
			"error", "self_deletion_attempt",
		).Warn("Admin attempted to delete their own account")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete your own account"})
		return
	}

	// Delete user from database
	db := database.GetDB()
	if err := db.DeleteUser(c.Request.Context(), id); err != nil {
		if err == database.ErrUserNotFound {
			logging.Auth.WithFields(
				"admin_user_id", adminUser.ID,
				"target_user_id", id,
				"error", "user_not_found",
			).Warn("Deletion failed: user not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		logging.DB.WithFields(
			"admin_user_id", adminUser.ID,
			"target_user_id", id,
			"error", err.Error(),
		).Error("Database error when deleting user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	logging.Auth.WithFields(
		"admin_user_id", adminUser.ID,
		"username", adminUser.Username,
		"target_user_id", id,
	).Info("User deleted successfully")

	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}

// GrantUserPermissionsHandler grants permissions to a user (admin only).
//
// @Summary      Grant permissions to user
// @Description  Adds specific permissions to a user
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id        path      integer                 true  "User ID"
// @Param        request   body      ModifyPermissionsRequest true  "Permissions to grant"
// @Success      200       {object}  map[string]interface{}  "Updated user permissions"
// @Failure      400       {object}  map[string]string       "Invalid request"
// @Failure      401       {object}  map[string]string       "Authentication required"
// @Failure      403       {object}  map[string]string       "Permission denied"
// @Failure      404       {object}  map[string]string       "User not found"
// @Failure      500       {object}  map[string]string       "Server error"
// @Router       /users/{id}/permissions/grant [post]
func GrantUserPermissionsHandler(c *gin.Context) {
	// Get admin user
	adminUser, _ := auth.GetCurrentUser(c)

	// Get user ID
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logging.API.InvalidRequest.WithFields(
			"admin_user_id", adminUser.ID,
			"requested_id", idStr,
			"error", "invalid_id_format",
		).Warn("Invalid user ID format in grant permissions request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Parse request
	var req ModifyPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logging.API.InvalidRequest.WithFields(
			"admin_user_id", adminUser.ID,
			"target_user_id", id,
			"error", err.Error(),
		).Warn("Invalid permission grant request format")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get target user
	db := database.GetDB()
	user, err := db.GetUserByID(c.Request.Context(), id)
	if err != nil {
		if err == database.ErrUserNotFound {
			logging.Auth.WithFields(
				"admin_user_id", adminUser.ID,
				"target_user_id", id,
				"error", "user_not_found",
			).Warn("Permission grant failed: target user not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		logging.DB.WithFields(
			"admin_user_id", adminUser.ID,
			"target_user_id", id,
			"error", err.Error(),
		).Error("Database error when retrieving user for permission grant")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Apply permissions
	oldPermissions := user.Permissions
	for _, perm := range req.Permissions {
		user.Permissions |= perm.Permission
	}

	// Save updated permissions
	if err := db.UpdateUser(c.Request.Context(), user); err != nil {
		logging.DB.WithFields(
			"admin_user_id", adminUser.ID,
			"target_user_id", id,
			"error", err.Error(),
		).Error("Database error when updating user permissions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	logging.Auth.WithFields(
		"admin_user_id", adminUser.ID,
		"admin_username", adminUser.Username,
		"target_user_id", id,
		"old_permissions", oldPermissions,
		"new_permissions", user.Permissions,
	).Info("User permissions updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"user_id":         user.ID,
		"username":        user.Username,
		"old_permissions": oldPermissions,
		"new_permissions": user.Permissions,
	})
}

// RevokeUserPermissionsHandler revokes permissions from a user (admin only).
//
// @Summary      Revoke permissions from user
// @Description  Removes specific permissions from a user
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id        path      integer                 true  "User ID"
// @Param        request   body      ModifyPermissionsRequest true  "Permissions to revoke"
// @Success      200       {object}  map[string]interface{}  "Updated user permissions"
// @Failure      400       {object}  map[string]string       "Invalid request"
// @Failure      401       {object}  map[string]string       "Authentication required"
// @Failure      403       {object}  map[string]string       "Permission denied"
// @Failure      404       {object}  map[string]string       "User not found"
// @Failure      500       {object}  map[string]string       "Server error"
// @Router       /users/{id}/permissions/revoke [post]
func RevokeUserPermissionsHandler(c *gin.Context) {
	// Get admin user
	adminUser, _ := auth.GetCurrentUser(c)

	// Get user ID
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logging.API.InvalidRequest.WithFields(
			"admin_user_id", adminUser.ID,
			"requested_id", idStr,
			"error", "invalid_id_format",
		).Warn("Invalid user ID format in revoke permissions request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Parse request
	var req ModifyPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logging.API.InvalidRequest.WithFields(
			"admin_user_id", adminUser.ID,
			"target_user_id", id,
			"error", err.Error(),
		).Warn("Invalid permission revoke request format")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get target user
	db := database.GetDB()
	user, err := db.GetUserByID(c.Request.Context(), id)
	if err != nil {
		if err == database.ErrUserNotFound {
			logging.Auth.WithFields(
				"admin_user_id", adminUser.ID,
				"target_user_id", id,
				"error", "user_not_found",
			).Warn("Permission revoke failed: target user not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		logging.DB.WithFields(
			"admin_user_id", adminUser.ID,
			"target_user_id", id,
			"error", err.Error(),
		).Error("Database error when retrieving user for permission revoke")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Revoke permissions
	oldPermissions := user.Permissions
	for _, perm := range req.Permissions {
		user.Permissions &= ^perm.Permission // Retire la permission avec NOT et AND
	}

	// Save updated permissions
	if err := db.UpdateUser(c.Request.Context(), user); err != nil {
		logging.DB.WithFields(
			"admin_user_id", adminUser.ID,
			"target_user_id", id,
			"error", err.Error(),
		).Error("Database error when updating user permissions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	logging.Auth.WithFields(
		"admin_user_id", adminUser.ID,
		"admin_username", adminUser.Username,
		"target_user_id", id,
		"old_permissions", oldPermissions,
		"new_permissions", user.Permissions,
	).Info("User permissions revoked successfully")

	c.JSON(http.StatusOK, gin.H{
		"user_id":         user.ID,
		"username":        user.Username,
		"old_permissions": oldPermissions,
		"new_permissions": user.Permissions,
	})
}

// GetPermissionsMapHandler returns a mapping of permission values to their names.
//
// @Summary      Get permissions map
// @Description  Returns a mapping of permission values to their names
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  map[string]int64  "Permissions map"
// @Failure      401  {object}  map[string]string "Authentication required"
// @Router       /permissions [get]
func GetPermissionsMapHandler(c *gin.Context) {
	// Return a map of permission names to their values
	permissionsMap := map[string]int64{
		"PermAdmin":         database.PermAdmin,
		"PermCreateServer":  database.PermCreateServer,
		"PermDeleteServer":  database.PermDeleteServer,
		"PermStartServer":   database.PermStartServer,
		"PermStopServer":    database.PermStopServer,
		"PermRestartServer": database.PermRestartServer,
		"PermExecCommand":   database.PermExecCommand,
		"PermViewServer":    database.PermViewServer,
	}

	// Add permissions for database access
	permissionsMap["PermOperator"] = database.PermOperator
	permissionsMap["PermAll"] = database.PermAll
	permissionsMap["PermReadOnly"] = database.PermReadOnly

	c.JSON(http.StatusOK, permissionsMap)
}
