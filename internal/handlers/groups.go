package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// CreateGroupRequest represents the request to create a group
type CreateGroupRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// CreateGroupResponse represents the response after creating a group
type CreateGroupResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Group   map[string]interface{} `json:"group,omitempty"`
}

// UpdateGroupRequest represents the request to update an existing group
type UpdateGroupRequest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// Generic response for update/delete
type GroupActionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetGroupsResponse represents the response for getting groups
type GetGroupsResponse struct {
	Success bool                     `json:"success"`
	Groups  []map[string]interface{} `json:"groups"`
	Total   int                      `json:"total"`
}

// JoinGroupResponse represents the response for joining a group
type JoinGroupResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetGroupMembersResponse represents the response for getting group members
type GetGroupMembersResponse struct {
	Success  bool                     `json:"success"`
	Members  []map[string]interface{} `json:"members"`
	Total    int                      `json:"total"`
}

// GetGroupMessagesResponse represents the response for getting group messages
type GetGroupMessagesResponse struct {
	Success  bool                     `json:"success"`
	Messages []map[string]interface{} `json:"messages"`
	HasMore  bool                     `json:"has_more"`
	Total    int                      `json:"total"`
}

// SendGroupMessageRequest represents the request to send a message
type SendGroupMessageRequest struct {
	Message string `json:"message"`
}

// SendGroupMessageResponse represents the response after sending a message
type SendGroupMessageResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Msg     map[string]interface{} `json:"msg,omitempty"`
}

// getCurrentUser gets the current user from session token (optional - returns nil if not authenticated)
func getCurrentUser(r *http.Request) (*uuid.UUID, error) {
	token := extractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		return nil, nil // Not authenticated, but that's okay for viewing
	}
	
	userID, ok, err := services.ValidateSession(token)
	if err != nil || !ok {
		return nil, nil // Invalid session, but that's okay for viewing
	}
	
	return &userID, nil
}

// CreateGroup handles creating a new group (requires authentication)
func CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateGroupResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.Name) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateGroupResponse{
			Success: false,
			Message: "Group name is required",
		})
		return
	}

	// Normalize and validate tags (optional)
	var tags []string
	for _, t := range req.Tags {
		trimmed := strings.TrimSpace(t)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}

	// Ensure group name is unique (case-insensitive)
	var exists bool
	if err := database.PostgresDB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM groups WHERE LOWER(name) = LOWER($1))
	`, req.Name).Scan(&exists); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CreateGroupResponse{
			Success: false,
			Message: "Failed to validate group name",
		})
		return
	}
	if exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(CreateGroupResponse{
			Success: false,
			Message: "A group with this name already exists",
		})
		return
	}

	// Get current user (required for creating groups)
	userID, err := getCurrentUser(r)
	if err != nil || userID == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(CreateGroupResponse{
			Success: false,
			Message: "You must be signed in to create a group",
		})
		return
	}

	// Create group
	groupID := uuid.New()
	now := time.Now()
	slug := services.GenerateUniqueGroupSlug(req.Name)
	
	_, err = database.PostgresDB.Exec(`
		INSERT INTO groups (id, created_at, updated_at, name, slug, description, created_by, is_public, member_count, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, groupID, now, now, req.Name, slug, req.Description, *userID, true, 1, pq.StringArray(tags))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CreateGroupResponse{
			Success: false,
			Message: "Failed to create group",
		})
		return
	}

	// Add creator as member (admin role). Creator must NOT need to manually join.
	_, err = database.PostgresDB.Exec(`
		INSERT INTO group_members (id, group_id, user_id, role, joined_at)
		VALUES (gen_random_uuid(), $1, $2, 'admin', $3)
		ON CONFLICT (group_id, user_id) DO NOTHING
	`, groupID, *userID, now)
	if err != nil {
		// Log error but continue - group is created
	}

	// Get username for response
	username, _ := services.GetUsernameByID(userID.String())

	groupMap := map[string]interface{}{
		"id":           groupID.String(),
		"name":         req.Name,
		"slug":         slug,
		"description":  req.Description,
		"created_by":   username,
		"created_at":   now,
		"member_count": 1,
		"is_public":    true,
		"tags":         tags,
		"is_creator":   true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(CreateGroupResponse{
		Success: true,
		Message: "Group created successfully",
		Group:   groupMap,
	})
}

// UpdateGroup allows the creator to edit name/description/tags of a group
func UpdateGroup(w http.ResponseWriter, r *http.Request) {
	var req UpdateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	if strings.TrimSpace(req.ID) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Group ID is required",
		})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Group name is required",
		})
		return
	}

	groupID, err := uuid.Parse(req.ID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Invalid group ID",
		})
		return
	}

	// Get current user (must be creator)
	userID, err := getCurrentUser(r)
	if err != nil || userID == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "You must be signed in to update a group",
		})
		return
	}

	// Verify user is creator
	var createdBy uuid.UUID
	err = database.PostgresDB.QueryRow(`
		SELECT created_by FROM groups WHERE id = $1
	`, groupID).Scan(&createdBy)
	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(GroupActionResponse{
				Success: false,
				Message: "Group not found",
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Failed to load group",
		})
		return
	}
	if createdBy != *userID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Only the group creator can update this group",
		})
		return
	}

	// Normalize tags
	var tags []string
	for _, t := range req.Tags {
		trimmed := strings.TrimSpace(t)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}

	// Ensure new name is unique across other groups
	var exists bool
	if err := database.PostgresDB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM groups WHERE LOWER(name) = LOWER($1) AND id <> $2)
	`, req.Name, groupID).Scan(&exists); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Failed to validate group name",
		})
		return
	}
	if exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "A group with this name already exists",
		})
		return
	}

	_, err = database.PostgresDB.Exec(`
		UPDATE groups
		SET name = $1,
		    description = $2,
		    tags = $3,
		    updated_at = $4
		WHERE id = $5 AND created_by = $6
	`, req.Name, req.Description, pq.StringArray(tags), time.Now(), groupID, *userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Failed to update group",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GroupActionResponse{
		Success: true,
		Message: "Group updated successfully",
	})
}

// DeleteGroup allows the creator to delete a group
func DeleteGroup(w http.ResponseWriter, r *http.Request) {
	groupIDStr := r.URL.Query().Get("group_id")
	if strings.TrimSpace(groupIDStr) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Group ID is required",
		})
		return
	}

	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Invalid group ID",
		})
		return
	}

	userID, err := getCurrentUser(r)
	if err != nil || userID == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "You must be signed in to delete a group",
		})
		return
	}

	// Delete only if creator matches
	res, err := database.PostgresDB.Exec(`
		DELETE FROM groups WHERE id = $1 AND created_by = $2
	`, groupID, *userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Failed to delete group",
		})
		return
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Only the group creator can delete this group",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GroupActionResponse{
		Success: true,
		Message: "Group deleted successfully",
	})
}

// GetGroups handles getting all public groups (requires authentication).
func GetGroups(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := getCurrentUser(r)
	if err != nil || currentUserID == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(GetGroupsResponse{
			Success: false,
			Groups:  []map[string]interface{}{},
			Total:   0,
		})
		return
	}

	// Get query parameters
	limitStr := r.URL.Query().Get("limit")
	skipStr := r.URL.Query().Get("skip")
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))

	// Parse limit (default: 50)
	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Parse skip (default: 0)
	skip := 0
	if skipStr != "" {
		if parsedSkip, err := strconv.Atoi(skipStr); err == nil && parsedSkip >= 0 {
			skip = parsedSkip
		}
	}

	// Build dynamic WHERE clause
	conditions := []string{"g.is_public = TRUE"}
	args := []interface{}{}
	argIdx := 1

	if search != "" {
		conditions = append(conditions, "LOWER(g.name) LIKE $"+strconv.Itoa(argIdx))
		args = append(args, "%"+strings.ToLower(search)+"%")
		argIdx++
	}
	if tag != "" {
		conditions = append(conditions, "$"+strconv.Itoa(argIdx)+" = ANY(g.tags)")
		args = append(args, tag)
		argIdx++
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	// Count total groups
	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM groups g
		` + whereClause
	err = database.PostgresDB.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetGroupsResponse{
			Success: false,
			Groups:  []map[string]interface{}{},
			Total:   0,
		})
		return
	}

	// Get groups with pagination
	selectQuery := `
		SELECT g.id, g.name, g.slug, g.description, g.created_at, g.member_count, g.created_by,
		       u.username, COALESCE(g.tags, '{}'::text[])
		FROM groups g
		LEFT JOIN users u ON g.created_by = u.id
		` + whereClause + `
		ORDER BY g.created_at DESC
		LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)

	selectArgs := append(append([]interface{}{}, args...), limit, skip)

	rows, err := database.PostgresDB.Query(selectQuery, selectArgs...)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetGroupsResponse{
			Success: false,
			Groups:  []map[string]interface{}{},
			Total:   0,
		})
		return
	}
	defer rows.Close()

	var groups []map[string]interface{}
	for rows.Next() {
		var groupID, createdBy uuid.UUID
		var name, description, slug sql.NullString
		var createdAt time.Time
		var memberCount int
		var username sql.NullString
		var tags pq.StringArray

		err := rows.Scan(&groupID, &name, &slug, &description, &createdAt, &memberCount, &createdBy, &username, &tags)
		if err != nil {
			continue
		}

		groupMap := map[string]interface{}{
			"id":           groupID.String(),
			"name":         name.String,
			"slug":         slug.String,
			"description": description.String,
			"created_at":   createdAt,
			"member_count": memberCount,
			"created_by":   username.String,
			"is_public":    true,
			"tags":         []string(tags),
		}
		if createdBy == *currentUserID {
			groupMap["is_creator"] = true
		} else {
			groupMap["is_creator"] = false
		}

		groups = append(groups, groupMap)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetGroupsResponse{
		Success: true,
		Groups:  groups,
		Total:   total,
	})
}

// JoinGroup handles joining a group (requires authentication)
func JoinGroup(w http.ResponseWriter, r *http.Request) {
	// Get group ID from URL
	groupIDStr := r.URL.Query().Get("group_id")
	if groupIDStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(JoinGroupResponse{
			Success: false,
			Message: "Group ID is required",
		})
		return
	}

	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(JoinGroupResponse{
			Success: false,
			Message: "Invalid group ID",
		})
		return
	}

	// Get current user (required for joining)
	userID, err := getCurrentUser(r)
	if err != nil || userID == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(JoinGroupResponse{
			Success: false,
			Message: "You must be signed in to join a group",
		})
		return
	}

	// Check if group exists and is public
	var isPublic bool
	err = database.PostgresDB.QueryRow(`
		SELECT is_public FROM groups WHERE id = $1
	`, groupID).Scan(&isPublic)
	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(JoinGroupResponse{
				Success: false,
				Message: "Group not found",
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(JoinGroupResponse{
			Success: false,
			Message: "Failed to check group",
		})
		return
	}

	if !isPublic {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(JoinGroupResponse{
			Success: false,
			Message: "This group is not public",
		})
		return
	}

	// Check if user is already a member
	var existingMemberID uuid.UUID
	err = database.PostgresDB.QueryRow(`
		SELECT id FROM group_members WHERE group_id = $1 AND user_id = $2
	`, groupID, *userID).Scan(&existingMemberID)
	if err == nil {
		// Already a member
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(JoinGroupResponse{
			Success: true,
			Message: "You are already a member of this group",
		})
		return
	}

	// Add user as member
	_, err = database.PostgresDB.Exec(`
		INSERT INTO group_members (id, group_id, user_id, joined_at)
		VALUES (gen_random_uuid(), $1, $2, $3)
	`, groupID, *userID, time.Now())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(JoinGroupResponse{
			Success: false,
			Message: "Failed to join group",
		})
		return
	}

	// Update member count
	_, err = database.PostgresDB.Exec(`
		UPDATE groups SET member_count = member_count + 1 WHERE id = $1
	`, groupID)
	if err != nil {
		// Log error but continue
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JoinGroupResponse{
		Success: true,
		Message: "Successfully joined group",
	})
}

// RemoveMember allows the group creator to remove a member (cannot remove self)
func RemoveMember(w http.ResponseWriter, r *http.Request) {
	groupIDStr := r.URL.Query().Get("group_id")
	memberUserIDStr := r.URL.Query().Get("user_id")
	if strings.TrimSpace(groupIDStr) == "" || strings.TrimSpace(memberUserIDStr) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "group_id and user_id are required",
		})
		return
	}

	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Invalid group ID",
		})
		return
	}
	memberUserID, err := uuid.Parse(memberUserIDStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Invalid user ID",
		})
		return
	}

	creatorID, err := getCurrentUser(r)
	if err != nil || creatorID == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "You must be signed in to remove members",
		})
		return
	}

	// Creator cannot remove themselves (they can delete the group instead)
	if *creatorID == memberUserID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "You cannot remove yourself. Delete the group if you want to leave.",
		})
		return
	}

	// Verify requester is the group creator
	var createdBy uuid.UUID
	err = database.PostgresDB.QueryRow(`
		SELECT created_by FROM groups WHERE id = $1
	`, groupID).Scan(&createdBy)
	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(GroupActionResponse{
				Success: false,
				Message: "Group not found",
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Failed to load group",
		})
		return
	}
	if createdBy != *creatorID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Only the group creator can remove members",
		})
		return
	}

	res, err := database.PostgresDB.Exec(`
		DELETE FROM group_members WHERE group_id = $1 AND user_id = $2
	`, groupID, memberUserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Failed to remove member",
		})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(GroupActionResponse{
			Success: false,
			Message: "Member not found in this group",
		})
		return
	}

	_, _ = database.PostgresDB.Exec(`
		UPDATE groups SET member_count = member_count - 1 WHERE id = $1
	`, groupID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GroupActionResponse{
		Success: true,
		Message: "Member removed successfully",
	})
}

// GetGroupMembers handles getting members of a group (requires authentication).
func GetGroupMembers(w http.ResponseWriter, r *http.Request) {
	userID, err := getCurrentUser(r)
	if err != nil || userID == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(GetGroupMembersResponse{
			Success: false,
			Members: []map[string]interface{}{},
			Total:   0,
		})
		return
	}
	_ = userID // Auth check only

	// Get group ID from URL
	groupIDStr := r.URL.Query().Get("group_id")
	if groupIDStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GetGroupMembersResponse{
			Success: false,
			Members: []map[string]interface{}{},
			Total:   0,
		})
		return
	}

	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GetGroupMembersResponse{
			Success: false,
			Members: []map[string]interface{}{},
			Total:   0,
		})
		return
	}

	// Count total members
	var total int
	err = database.PostgresDB.QueryRow(`
		SELECT COUNT(*) FROM group_members WHERE group_id = $1
	`, groupID).Scan(&total)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetGroupMembersResponse{
			Success: false,
			Members: []map[string]interface{}{},
			Total:   0,
		})
		return
	}

	// Get members
	rows, err := database.PostgresDB.Query(`
		SELECT gm.user_id, gm.joined_at, u.username
		FROM group_members gm
		LEFT JOIN users u ON gm.user_id = u.id
		WHERE gm.group_id = $1
		ORDER BY gm.joined_at ASC
	`, groupID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetGroupMembersResponse{
			Success: false,
			Members: []map[string]interface{}{},
			Total:   0,
		})
		return
	}
	defer rows.Close()

	var members []map[string]interface{}
	for rows.Next() {
		var userID uuid.UUID
		var joinedAt time.Time
		var username sql.NullString

		err := rows.Scan(&userID, &joinedAt, &username)
		if err != nil {
			continue
		}

		memberMap := map[string]interface{}{
			"user_id":   userID.String(),
			"username":  username.String,
			"joined_at": joinedAt,
		}

		members = append(members, memberMap)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetGroupMembersResponse{
		Success: true,
		Members: members,
		Total:   total,
	})
}

// GetGroupMessages handles getting messages from a group
func GetGroupMessages(w http.ResponseWriter, r *http.Request) {
	// Get group ID from URL
	groupIDStr := r.URL.Query().Get("group_id")
	if groupIDStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GetGroupMessagesResponse{
			Success:  false,
			Messages: []map[string]interface{}{},
			HasMore:  false,
			Total:    0,
		})
		return
	}

	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GetGroupMessagesResponse{
			Success:  false,
			Messages: []map[string]interface{}{},
			HasMore:  false,
			Total:    0,
		})
		return
	}

	// Get query parameters
	limitStr := r.URL.Query().Get("limit")
	skipStr := r.URL.Query().Get("skip")

	// Parse limit (default: 50)
	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Parse skip (default: 0)
	skip := 0
	if skipStr != "" {
		if parsedSkip, err := strconv.Atoi(skipStr); err == nil && parsedSkip >= 0 {
			skip = parsedSkip
		}
	}

	// Count total messages
	var total int
	err = database.PostgresDB.QueryRow(`
		SELECT COUNT(*) FROM group_messages WHERE group_id = $1
	`, groupID).Scan(&total)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetGroupMessagesResponse{
			Success:  false,
			Messages: []map[string]interface{}{},
			HasMore:  false,
			Total:    0,
		})
		return
	}

	// Get messages with pagination (newest first)
	rows, err := database.PostgresDB.Query(`
		SELECT gm.id, gm.message, gm.created_at, gm.user_id, u.username
		FROM group_messages gm
		LEFT JOIN users u ON gm.user_id = u.id
		WHERE gm.group_id = $1
		ORDER BY gm.created_at DESC
		LIMIT $2 OFFSET $3
	`, groupID, limit, skip)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetGroupMessagesResponse{
			Success:  false,
			Messages: []map[string]interface{}{},
			HasMore:  false,
			Total:    0,
		})
		return
	}
	defer rows.Close()

	var messages []map[string]interface{}
	for rows.Next() {
		var msgID, userID uuid.UUID
		var message string
		var createdAt time.Time
		var username sql.NullString

		err := rows.Scan(&msgID, &message, &createdAt, &userID, &username)
		if err != nil {
			continue
		}

		msgMap := map[string]interface{}{
			"id":         msgID.String(),
			"message":   message,
			"created_at": createdAt,
			"user_id":    userID.String(),
			"username":   username.String,
		}

		messages = append(messages, msgMap)
	}

	// Reverse messages to show oldest first (for chat UI)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	hasMore := skip+limit < total

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetGroupMessagesResponse{
		Success:  true,
		Messages: messages,
		HasMore:  hasMore,
		Total:    total,
	})
}

// SendGroupMessage handles sending a message to a group (requires authentication)
func SendGroupMessage(w http.ResponseWriter, r *http.Request) {
	// Get group ID from URL
	groupIDStr := r.URL.Query().Get("group_id")
	if groupIDStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SendGroupMessageResponse{
			Success: false,
			Message: "Group ID is required",
		})
		return
	}

	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SendGroupMessageResponse{
			Success: false,
			Message: "Invalid group ID",
		})
		return
	}

	var req SendGroupMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SendGroupMessageResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate message
	if req.Message == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SendGroupMessageResponse{
			Success: false,
			Message: "Message is required",
		})
		return
	}

	// Get current user (required for sending messages)
	userID, err := getCurrentUser(r)
	if err != nil || userID == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(SendGroupMessageResponse{
			Success: false,
			Message: "You must be signed in to send messages",
		})
		return
	}

	// Check if group exists
	var exists bool
	err = database.PostgresDB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM groups WHERE id = $1)
	`, groupID).Scan(&exists)
	if err != nil || !exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(SendGroupMessageResponse{
			Success: false,
			Message: "Group not found",
		})
		return
	}

	// Check if user is a member
	var isMember bool
	err = database.PostgresDB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2)
	`, groupID, *userID).Scan(&isMember)
	if err != nil || !isMember {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(SendGroupMessageResponse{
			Success: false,
			Message: "You must be a member of this group to send messages",
		})
		return
	}

	// Create message
	msgID := uuid.New()
	now := time.Now()
	
	_, err = database.PostgresDB.Exec(`
		INSERT INTO group_messages (id, created_at, group_id, user_id, message)
		VALUES ($1, $2, $3, $4, $5)
	`, msgID, now, groupID, *userID, req.Message)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SendGroupMessageResponse{
			Success: false,
			Message: "Failed to send message",
		})
		return
	}

	// Get username for response
	username, _ := services.GetUsernameByID(userID.String())

	msgMap := map[string]interface{}{
		"id":         msgID.String(),
		"message":    req.Message,
		"created_at": now,
		"user_id":    userID.String(),
		"username":   username,
		"group_id":   groupID.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(SendGroupMessageResponse{
		Success: true,
		Message: "Message sent successfully",
		Msg:     msgMap,
	})
}

// AdminGetAllGroups returns all groups (admin only). No is_public filter.
func AdminGetAllGroups(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}

	limitStr := r.URL.Query().Get("limit")
	skipStr := r.URL.Query().Get("skip")
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))

	limit := 100
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 200 {
			limit = parsedLimit
		}
	}
	skip := 0
	if skipStr != "" {
		if parsedSkip, err := strconv.Atoi(skipStr); err == nil && parsedSkip >= 0 {
			skip = parsedSkip
		}
	}

	conditions := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1
	if search != "" {
		conditions = append(conditions, "LOWER(g.name) LIKE $"+strconv.Itoa(argIdx))
		args = append(args, "%"+strings.ToLower(search)+"%")
		argIdx++
	}
	if tag != "" {
		conditions = append(conditions, "$"+strconv.Itoa(argIdx)+" = ANY(g.tags)")
		args = append(args, tag)
		argIdx++
	}
	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	var total int
	countQuery := `SELECT COUNT(*) FROM groups g ` + whereClause
	if err := database.PostgresDB.QueryRow(countQuery, args...).Scan(&total); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetGroupsResponse{Success: false, Groups: []map[string]interface{}{}, Total: 0})
		return
	}

	selectQuery := `
		SELECT g.id, g.name, g.slug, g.description, g.created_at, g.member_count, g.created_by,
		       u.username, COALESCE(g.tags, '{}'::text[])
		FROM groups g
		LEFT JOIN users u ON g.created_by = u.id
		` + whereClause + `
		ORDER BY g.created_at DESC
		LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	selectArgs := append(append([]interface{}{}, args...), limit, skip)

	rows, err := database.PostgresDB.Query(selectQuery, selectArgs...)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetGroupsResponse{Success: false, Groups: []map[string]interface{}{}, Total: 0})
		return
	}
	defer rows.Close()

	var groups []map[string]interface{}
	for rows.Next() {
		var groupID, createdBy uuid.UUID
		var name, description, slug sql.NullString
		var createdAt time.Time
		var memberCount int
		var username sql.NullString
		var tags pq.StringArray
		if err := rows.Scan(&groupID, &name, &slug, &description, &createdAt, &memberCount, &createdBy, &username, &tags); err != nil {
			continue
		}
		groupMap := map[string]interface{}{
			"id":           groupID.String(),
			"name":         name.String,
			"slug":         slug.String,
			"description": description.String,
			"created_at":   createdAt,
			"member_count": memberCount,
			"created_by":   username.String,
			"tags":         []string(tags),
		}
		groups = append(groups, groupMap)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetGroupsResponse{Success: true, Groups: groups, Total: total})
}

// AdminGetGroupMembers returns members of a group (admin only).
func AdminGetGroupMembers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}

	groupIDStr := r.URL.Query().Get("group_id")
	if strings.TrimSpace(groupIDStr) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GetGroupMembersResponse{Success: false, Members: []map[string]interface{}{}, Total: 0})
		return
	}

	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GetGroupMembersResponse{Success: false, Members: []map[string]interface{}{}, Total: 0})
		return
	}

	var total int
	if err := database.PostgresDB.QueryRow(`SELECT COUNT(*) FROM group_members WHERE group_id = $1`, groupID).Scan(&total); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetGroupMembersResponse{Success: false, Members: []map[string]interface{}{}, Total: 0})
		return
	}

	rows, err := database.PostgresDB.Query(`
		SELECT gm.user_id, gm.joined_at, u.username
		FROM group_members gm
		LEFT JOIN users u ON gm.user_id = u.id
		WHERE gm.group_id = $1
		ORDER BY gm.joined_at ASC
	`, groupID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetGroupMembersResponse{Success: false, Members: []map[string]interface{}{}, Total: 0})
		return
	}
	defer rows.Close()

	var members []map[string]interface{}
	for rows.Next() {
		var userID uuid.UUID
		var joinedAt time.Time
		var username sql.NullString
		if err := rows.Scan(&userID, &joinedAt, &username); err != nil {
			continue
		}
		members = append(members, map[string]interface{}{
			"user_id":   userID.String(),
			"username":  username.String,
			"joined_at": joinedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetGroupMembersResponse{Success: true, Members: members, Total: total})
}

// AdminDeleteGroup deletes a group (admin only). Can delete any group.
func AdminDeleteGroup(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}

	groupIDStr := r.URL.Query().Get("group_id")
	if strings.TrimSpace(groupIDStr) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GroupActionResponse{Success: false, Message: "group_id is required"})
		return
	}

	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GroupActionResponse{Success: false, Message: "Invalid group ID"})
		return
	}

	res, err := database.PostgresDB.Exec(`DELETE FROM groups WHERE id = $1`, groupID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GroupActionResponse{Success: false, Message: "Failed to delete group"})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(GroupActionResponse{Success: false, Message: "Group not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GroupActionResponse{Success: true, Message: "Group deleted successfully"})
}

