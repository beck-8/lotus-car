package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/minerdao/lotus-car/db"
)

type CarFileResponse struct {
	ID        string `json:"id"`
	CommP     string `json:"commp"`
	DataCid   string `json:"data_cid"`
	PieceCid  string `json:"piece_cid"`
	PieceSize int64  `json:"piece_size"`
	CarSize   int64  `json:"car_size"`
	FilePath  string `json:"file_path"`
	CreatedAt string `json:"created_at"`
}

type APIServer struct {
	db *db.Database
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func NewAPIServer(config *db.DBConfig) (*APIServer, error) {
	database, err := db.InitDB(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}
	return &APIServer{db: database}, nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}

func (s *APIServer) ListCarFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET method is allowed")
		return
	}

	files, err := s.db.ListCarFiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list car files: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, files)
}

func (s *APIServer) GetCarFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET method is allowed")
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "id parameter is required")
		return
	}

	// UUID validation
	if _, err := uuid.Parse(idStr); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid UUID format: %v", err))
		return
	}

	file, err := s.db.GetCarFile(idStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get car file: %v", err))
		return
	}

	if file == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("car file with id %s not found", idStr))
		return
	}

	writeJSON(w, http.StatusOK, file)
}

func (s *APIServer) DeleteCarFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "only DELETE method is allowed")
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "id parameter is required")
		return
	}

	// UUID validation
	if _, err := uuid.Parse(idStr); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid UUID format: %v", err))
		return
	}

	err := s.db.DeleteCarFile(idStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete car file: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("car file %s deleted successfully", idStr)})
}

func (s *APIServer) SearchCarFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET method is allowed")
		return
	}

	query := r.URL.Query()
	searchParams := db.SearchParams{
		CommP:    query.Get("commp"),
		DataCid:  query.Get("data_cid"),
		PieceCid: query.Get("piece_cid"),
	}

	// Validate that at least one search parameter is provided
	if searchParams.CommP == "" && searchParams.DataCid == "" && searchParams.PieceCid == "" {
		writeError(w, http.StatusBadRequest, "at least one search parameter (commp, data_cid, or piece_cid) is required")
		return
	}

	files, err := s.db.SearchCarFiles(searchParams)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to search car files: %v", err))
		return
	}

	if len(files) == 0 {
		// Return empty array instead of null for consistency
		writeJSON(w, http.StatusOK, []db.CarFile{})
		return
	}

	writeJSON(w, http.StatusOK, files)
}

// UpdateDealStatus 更新订单状态的处理函数
func (s *APIServer) UpdateDealStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "only PUT method is allowed")
		return
	}

	// 获取并验证 UUID
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "id parameter is required")
		return
	}

	if _, err := uuid.Parse(idStr); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid UUID format: %v", err))
		return
	}

	// 获取并验证状态
	status := r.URL.Query().Get("status")
	if status == "" {
		writeError(w, http.StatusBadRequest, "status parameter is required")
		return
	}

	// 验证状态值是否有效
	dealStatus := db.DealStatus(status)
	if dealStatus != db.DealStatusPending &&
		dealStatus != db.DealStatusSuccess &&
		dealStatus != db.DealStatusFailed {
		writeError(w, http.StatusBadRequest, "invalid status value, must be one of: pending, success, failed")
		return
	}

	// 更新状态
	err := s.db.UpdateDealStatus(idStr, dealStatus, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update deal status: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("deal status for car file %s updated to %s", idStr, status),
	})
}
