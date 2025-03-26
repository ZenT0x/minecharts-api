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

	logging.WithFields(
		logging.F("admin_user_id", adminUser.ID),
		logging.F("username", adminUser.Username),
		logging.F("remote_ip", c.ClientIP()),
	).Info("Admin requesting list of all users")

	db := database.GetDB()
	users, err := db.ListUsers(c.Request.Context())
	if err != nil {
		logging.WithFields(
			logging.F("admin_user_id", adminUser.ID),
			logging.F("error", err.Error()),
		).Error("Failed to list users from database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}

	logging.WithFields(
		logging.F("admin_user_id", adminUser.ID),
		logging.F("user_count", len(users)),
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Get user ID from URL parameter
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Users can only view their own details unless they're an admin
	if !currentUser.IsAdmin() && currentUser.ID != id {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	// Get user from database
	db := database.GetDB()
	user, err := db.GetUserByID(c.Request.Context(), id)
	if err != nil {
		if err == database.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Get user ID from URL parameter
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Users can only update their own details unless they're an admin
	isAdmin := currentUser.IsAdmin()
	isSelf := currentUser.ID == id

	if !isAdmin && !isSelf {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	// Get user from database
	db := database.GetDB()
	user, err := db.GetUserByID(c.Request.Context(), id)
	if err != nil {
		if err == database.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Parse update request
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Apply updates
	if req.Username != nil {
		user.Username = *req.Username
	}

	if req.Email != nil {
		user.Email = *req.Email
	}

	if req.Password != nil {
		// Only admins or the user themselves can change passwords
		if !isAdmin && !isSelf {
			c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
			return
		}

		passwordHash, err := auth.HashPassword(*req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		user.PasswordHash = passwordHash
	}

	if req.Permissions != nil {
		// Only admins can change permissions
		if !isAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only administrators can modify permissions"})
			return
		}
		user.Permissions = *req.Permissions
	}

	if req.Active != nil {
		// Only admins can change active status
		if !isAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only administrators can change account status"})
			return
		}
		user.Active = *req.Active
	}

	// Update user in database
	if err := db.UpdateUser(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

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
		logging.WithFields(
			logging.F("admin_user_id", adminUser.ID),
			logging.F("user_id_param", idStr),
			logging.F("error", "invalid_id_format"),
		).Warn("Invalid user ID format in delete request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	logging.WithFields(
		logging.F("admin_user_id", adminUser.ID),
		logging.F("username", adminUser.Username),
		logging.F("target_user_id", id),
		logging.F("remote_ip", c.ClientIP()),
	).Info("Admin attempting to delete user")

	// Don't allow admins to delete themselves
	if adminUser.ID == id {
		logging.WithFields(
			logging.F("admin_user_id", adminUser.ID),
			logging.F("error", "self_deletion_attempt"),
		).Warn("Admin attempted to delete their own account")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete your own account"})
		return
	}

	// Delete user from database
	db := database.GetDB()
	if err := db.DeleteUser(c.Request.Context(), id); err != nil {
		if err == database.ErrUserNotFound {
			logging.WithFields(
				logging.F("admin_user_id", adminUser.ID),
				logging.F("target_user_id", id),
				logging.F("error", "user_not_found"),
			).Warn("Deletion failed: user not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		logging.WithFields(
			logging.F("admin_user_id", adminUser.ID),
			logging.F("target_user_id", id),
			logging.F("error", err.Error()),
		).Error("Database error when deleting user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	logging.WithFields(
		logging.F("admin_user_id", adminUser.ID),
		logging.F("username", adminUser.Username),
		logging.F("target_user_id", id),
	).Info("User deleted successfully")

	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}
