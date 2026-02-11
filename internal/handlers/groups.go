package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/google/uuid"
)

// CreateGroupRequest represents the request to create a group
type CreateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CreateGroupResponse represents the response after creating a group
type CreateGroupResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Group   map[string]interface{} `json:"group,omitempty"`
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
	if req.Name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateGroupResponse{
			Success: false,
			Message: "Group name is required",
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
	
	_, err = database.PostgresDB.Exec(`
		INSERT INTO groups (id, created_at, updated_at, name, description, created_by, is_public, member_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, groupID, now, now, req.Name, req.Description, *userID, true, 1)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CreateGroupResponse{
			Success: false,
			Message: "Failed to create group",
		})
		return
	}

	// Add creator as member
	_, err = database.PostgresDB.Exec(`
		INSERT INTO group_members (id, group_id, user_id, joined_at)
		VALUES (gen_random_uuid(), $1, $2, $3)
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
		"description": req.Description,
		"created_by":   username,
		"created_at":   now,
		"member_count": 1,
		"is_public":    true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(CreateGroupResponse{
		Success: true,
		Message: "Group created successfully",
		Group:   groupMap,
	})
}

// GetGroups handles getting all public groups
func GetGroups(w http.ResponseWriter, r *http.Request) {
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

	// Count total groups
	var total int
	err := database.PostgresDB.QueryRow(`
		SELECT COUNT(*) FROM groups WHERE is_public = TRUE
	`).Scan(&total)
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
	rows, err := database.PostgresDB.Query(`
		SELECT g.id, g.name, g.description, g.created_at, g.member_count, g.created_by,
		       u.username
		FROM groups g
		LEFT JOIN users u ON g.created_by = u.id
		WHERE g.is_public = TRUE
		ORDER BY g.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, skip)
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
		var name, description sql.NullString
		var createdAt time.Time
		var memberCount int
		var username sql.NullString

		err := rows.Scan(&groupID, &name, &description, &createdAt, &memberCount, &createdBy, &username)
		if err != nil {
			continue
		}

		groupMap := map[string]interface{}{
			"id":           groupID.String(),
			"name":         name.String,
			"description": description.String,
			"created_at":   createdAt,
			"member_count": memberCount,
			"created_by":   username.String,
			"is_public":    true,
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

// GetGroupMembers handles getting members of a group
func GetGroupMembers(w http.ResponseWriter, r *http.Request) {
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

