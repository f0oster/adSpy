package web

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"f0oster/adspy/database/sqlcgen"
	"f0oster/adspy/web/sddiff"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Response types for JSON serialization

type ObjectResponse struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	DN        string  `json:"dn"`
	UpdatedAt string  `json:"updated_at"`
	DeletedAt *string `json:"deleted_at,omitempty"`
}

type ObjectListResponse struct {
	Objects []ObjectResponse `json:"objects"`
	Total   int64            `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}

type TimelineEntry struct {
	USNChanged int64           `json:"usn_changed"`
	Timestamp  string          `json:"timestamp"`
	Snapshot   json.RawMessage `json:"snapshot"`
	ModifiedBy string          `json:"modified_by,omitempty"`
}

type AttributeChange struct {
	SchemaID       string          `json:"schema_id"`
	Attribute      string          `json:"attribute"`
	OldValue       json.RawMessage `json:"old_value"`
	NewValue       json.RawMessage `json:"new_value"`
	Timestamp      string          `json:"timestamp"`
	IsSingleValued bool            `json:"is_single_valued"`
}

type SDDiffRequest struct {
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func parseUUID(s string) (pgtype.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

func formatTimestamp(ts pgtype.Timestamp) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.Format("2006-01-02T15:04:05Z")
}

func formatUUID(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}

// Handlers

func (s *Server) handleListObjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	// Parse query parameters
	typeFilter := q.Get("type")
	dnSearch := q.Get("search")
	limit := 50
	offset := 0

	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := q.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	queries := sqlcgen.New(s.db.Pool())

	// Get objects
	rows, err := queries.ListObjectsForWeb(ctx, sqlcgen.ListObjectsForWebParams{
		Column1: typeFilter,
		Column2: dnSearch,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list objects")
		return
	}

	// Get total count
	total, err := queries.CountObjectsForWeb(ctx, sqlcgen.CountObjectsForWebParams{
		Column1: typeFilter,
		Column2: dnSearch,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to count objects")
		return
	}

	// Convert to response format
	objects := make([]ObjectResponse, 0, len(rows))
	for _, row := range rows {
		obj := ObjectResponse{
			ID:        formatUUID(row.ObjectID),
			Type:      row.ObjectType,
			DN:        row.Distinguishedname,
			UpdatedAt: formatTimestamp(row.UpdatedAt),
		}
		if row.DeletedAt.Valid {
			ts := formatTimestamp(row.DeletedAt)
			obj.DeletedAt = &ts
		}
		objects = append(objects, obj)
	}

	writeJSON(w, http.StatusOK, ObjectListResponse{
		Objects: objects,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	})
}

func (s *Server) handleGetObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")

	objectID, err := parseUUID(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid object ID")
		return
	}

	queries := sqlcgen.New(s.db.Pool())
	row, err := queries.GetObjectByID(ctx, objectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Object not found")
		return
	}

	obj := ObjectResponse{
		ID:        formatUUID(row.ObjectID),
		Type:      row.ObjectType,
		DN:        row.Distinguishedname,
		UpdatedAt: formatTimestamp(row.UpdatedAt),
	}
	if row.DeletedAt.Valid {
		ts := formatTimestamp(row.DeletedAt)
		obj.DeletedAt = &ts
	}

	writeJSON(w, http.StatusOK, obj)
}

func (s *Server) handleGetObjectTimeline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")

	objectID, err := parseUUID(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid object ID")
		return
	}

	queries := sqlcgen.New(s.db.Pool())
	rows, err := queries.GetObjectTimeline(ctx, objectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get timeline")
		return
	}

	timeline := make([]TimelineEntry, 0, len(rows))
	for _, row := range rows {
		entry := TimelineEntry{
			USNChanged: row.UsnChanged,
			Timestamp:  formatTimestamp(row.Timestamp),
			Snapshot:   row.AttributesSnapshot,
		}
		if row.ModifiedBy.Valid {
			entry.ModifiedBy = row.ModifiedBy.String
		}
		timeline = append(timeline, entry)
	}

	writeJSON(w, http.StatusOK, timeline)
}

func (s *Server) handleGetVersionChanges(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	usnStr := r.PathValue("usn")

	objectID, err := parseUUID(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid object ID")
		return
	}

	usn, err := strconv.ParseInt(usnStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid USN")
		return
	}

	queries := sqlcgen.New(s.db.Pool())
	rows, err := queries.GetVersionChanges(ctx, sqlcgen.GetVersionChangesParams{
		ObjectID:   objectID,
		UsnChanged: usn,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get changes")
		return
	}

	changes := make([]AttributeChange, 0, len(rows))
	for _, row := range rows {
		change := AttributeChange{
			SchemaID:       formatUUID(row.AttributeSchemaID),
			Attribute:      row.LdapDisplayName,
			OldValue:       row.OldValue,
			NewValue:       row.NewValue,
			Timestamp:      formatTimestamp(row.Timestamp),
			IsSingleValued: row.IsSingleValued,
		}
		changes = append(changes, change)
	}

	writeJSON(w, http.StatusOK, changes)
}

func (s *Server) handleSDDiff(w http.ResponseWriter, r *http.Request) {
	var req SDDiffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Decode base64 values
	oldBytes, err := base64.StdEncoding.DecodeString(req.OldValue)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid base64 for old_value: "+err.Error())
		return
	}

	newBytes, err := base64.StdEncoding.DecodeString(req.NewValue)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid base64 for new_value: "+err.Error())
		return
	}

	diff, err := sddiff.DiffSecurityDescriptors(oldBytes, newBytes, s.sidResolver)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to diff: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, diff)
}

func (s *Server) handleGetObjectTypes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	queries := sqlcgen.New(s.db.Pool())
	types, err := queries.GetObjectTypes(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get object types")
		return
	}

	writeJSON(w, http.StatusOK, types)
}
