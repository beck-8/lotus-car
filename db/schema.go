package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"github.com/minerdao/lotus-car/config"
)

type RawFileInfo struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	RelativePath string `json:"relative_path"`
}

// DealStatus 表示订单发送状态
type DealStatus string

// RegenerateStatus 表示重新生成状态
type RegenerateStatus string

const (
	DealStatusPending DealStatus = "pending" // 未发送或正在发送
	DealStatusSuccess DealStatus = "success" // 发送成功
	DealStatusFailed  DealStatus = "failed"  // 发送失败

	RegenerateStatusPending RegenerateStatus = "pending" // 未重新生成或正在重新生成
	RegenerateStatusSuccess RegenerateStatus = "success" // 重新生成成功
	RegenerateStatusFailed  RegenerateStatus = "failed"  // 重新生成失败
)

type Deal struct {
	UUID               string    `json:"uuid"` // Primary key
	StorageProvider    string    `json:"storage_provider"`
	ClientWallet       string    `json:"client_wallet"`
	PayloadCid         string    `json:"payload_cid"`
	CommP              string    `json:"commp"`
	StartEpoch         int64     `json:"start_epoch"`
	EndEpoch           int64     `json:"end_epoch"`
	ProviderCollateral float64   `json:"provider_collateral"`
	Status             string    `json:"status"` // For tracking deal status
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CarFile struct {
	ID               string           `json:"id"`
	CommP            string           `json:"commp"`
	DataCid          string           `json:"data_cid"`
	PieceCid         string           `json:"piece_cid"`
	PieceSize        uint64           `json:"piece_size"`
	CarSize          uint64           `json:"car_size"`
	FilePath         string           `json:"file_path"`
	RawFiles         string           `json:"raw_files"` // JSON string of []RawFileInfo
	DealStatus       DealStatus       `json:"deal_status"`
	DealTime         *time.Time       `json:"deal_time"`         // 发单时间
	DealError        string           `json:"deal_error"`        // 发单失败的错误信息
	DealID           *string          `json:"deal_id"`           // Reference to Deal UUID, nullable
	RegenerateStatus RegenerateStatus `json:"regenerate_status"` // 重新生成状态
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"` // 记录更新时间
}

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"password"` // 存储密码哈希
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SearchParams struct {
	CommP    string
	DataCid  string
	PieceCid string
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type Database struct {
	db *sql.DB
}

func NewDBConfig() *DBConfig {
	return &DBConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		DBName:   "lotus_car",
		SSLMode:  "disable",
	}
}

func InitDB(config *DBConfig) (*Database, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Test the connection
	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	// Create users table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create users table: %v", err)
	}

	// Create deals table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS deals (
			uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			storage_provider TEXT NOT NULL,
			client_wallet TEXT NOT NULL,
			payload_cid TEXT NOT NULL,
			commp TEXT NOT NULL,
			start_epoch BIGINT NOT NULL,
			end_epoch BIGINT NOT NULL,
			provider_collateral REAL NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create deals table: %v", err)
	}

	// Create files table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS files (
			id TEXT PRIMARY KEY,
			comm_p TEXT NOT NULL,
			data_cid TEXT NOT NULL,
			piece_cid TEXT NOT NULL,
			piece_size BIGINT NOT NULL,
			car_size BIGINT NOT NULL,
			file_path TEXT NOT NULL,
			raw_files TEXT NOT NULL,
			deal_status TEXT NOT NULL DEFAULT 'pending',
			deal_time TIMESTAMP WITH TIME ZONE,
			deal_error TEXT,
			deal_id UUID REFERENCES deals(uuid),
			regenerate_status TEXT NOT NULL DEFAULT 'pending',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create files table: %v", err)
	}

	// Drop the old car_files table if it exists
	_, err = db.Exec(`DROP TABLE IF EXISTS car_files`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to drop car_files table: %v", err)
	}

	return &Database{db: db}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) InsertFile(file *CarFile) error {
	// Generate UUID if not provided
	if file.ID == "" {
		u, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		file.ID = u.String()
	}

	// Set default deal status if not provided
	if file.DealStatus == "" {
		file.DealStatus = DealStatusPending
	}

	// Set default regenerate status if not provided
	if file.RegenerateStatus == "" {
		file.RegenerateStatus = RegenerateStatusPending
	}

	err := d.db.QueryRow(`
		INSERT INTO files (id, comm_p, data_cid, piece_cid, piece_size, car_size, file_path, raw_files, deal_status, deal_time, deal_error, deal_id, regenerate_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at, updated_at`,
		file.ID, file.CommP, file.DataCid, file.PieceCid, file.PieceSize, file.CarSize, file.FilePath, file.RawFiles, file.DealStatus, file.DealTime, file.DealError, file.DealID, file.RegenerateStatus,
	).Scan(&file.ID, &file.CreatedAt, &file.UpdatedAt)

	return err
}

func (d *Database) ListFiles() ([]CarFile, error) {
	rows, err := d.db.Query(`
		SELECT id, comm_p, data_cid, piece_cid, piece_size, car_size, file_path, raw_files, deal_status, deal_time, deal_error, deal_id, regenerate_status, created_at, updated_at
		FROM files
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []CarFile
	for rows.Next() {
		var file CarFile
		err := rows.Scan(
			&file.ID,
			&file.CommP,
			&file.DataCid,
			&file.PieceCid,
			&file.PieceSize,
			&file.CarSize,
			&file.FilePath,
			&file.RawFiles,
			&file.DealStatus,
			&file.DealTime,
			&file.DealError,
			&file.DealID,
			&file.RegenerateStatus,
			&file.CreatedAt,
			&file.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

func (d *Database) GetFile(id string) (*CarFile, error) {
	file := &CarFile{}
	err := d.db.QueryRow(`
		SELECT id, comm_p, data_cid, piece_cid, piece_size, car_size, file_path, raw_files, deal_status, deal_time, deal_error, deal_id, regenerate_status, created_at, updated_at
		FROM files
		WHERE id = $1
	`, id).Scan(
		&file.ID,
		&file.CommP,
		&file.DataCid,
		&file.PieceCid,
		&file.PieceSize,
		&file.CarSize,
		&file.FilePath,
		&file.RawFiles,
		&file.DealStatus,
		&file.DealTime,
		&file.DealError,
		&file.DealID,
		&file.RegenerateStatus,
		&file.CreatedAt,
		&file.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (d *Database) DeleteFile(id string) error {
	result, err := d.db.Exec("DELETE FROM files WHERE id = $1", id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("file with id %s not found", id)
	}

	return nil
}

func (d *Database) SearchFiles(params SearchParams) ([]CarFile, error) {
	query := `
		SELECT id, comm_p, data_cid, piece_cid, piece_size, car_size, file_path, raw_files, deal_status, deal_time, deal_error, deal_id, regenerate_status, created_at, updated_at 
		FROM files 
		WHERE 1=1
	`
	var args []interface{}
	argIndex := 1

	if params.CommP != "" {
		query += fmt.Sprintf(" AND comm_p = $%d", argIndex)
		args = append(args, params.CommP)
		argIndex++
	}

	if params.DataCid != "" {
		query += fmt.Sprintf(" AND data_cid = $%d", argIndex)
		args = append(args, params.DataCid)
		argIndex++
	}

	if params.PieceCid != "" {
		query += fmt.Sprintf(" AND piece_cid = $%d", argIndex)
		args = append(args, params.PieceCid)
		argIndex++
	}

	query += " ORDER BY id DESC"

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []CarFile
	for rows.Next() {
		var file CarFile
		err := rows.Scan(
			&file.ID,
			&file.CommP,
			&file.DataCid,
			&file.PieceCid,
			&file.PieceSize,
			&file.CarSize,
			&file.FilePath,
			&file.RawFiles,
			&file.DealStatus,
			&file.DealTime,
			&file.DealError,
			&file.DealID,
			&file.RegenerateStatus,
			&file.CreatedAt,
			&file.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}

	return files, nil
}

func (d *Database) DB() *sql.DB {
	return d.db
}

func (d *Database) UpdateDealSentStatus(id string, status DealStatus, dealUUID string) error {
	now := time.Now()
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Update files table
	result, err := tx.Exec(`
		UPDATE files 
		SET deal_status = $1, deal_time = $2, deal_id = $3, updated_at = $4
		WHERE id = $5`,
		status, now, dealUUID, now, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update files: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("car file with id %s not found", id)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func (d *Database) GetUserByUsername(username string) (*User, error) {
	user := &User{}
	err := d.db.QueryRow(`
		SELECT id, username, password, created_at, updated_at
		FROM users
		WHERE username = $1
	`, username).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %v", err)
	}
	return string(bytes), nil
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (d *Database) CreateUser(username, password string) error {
	// 检查用户名是否已存在
	existingUser, err := d.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("failed to check existing user: %v", err)
	}
	if existingUser != nil {
		return fmt.Errorf("username %s already exists", username)
	}

	// 对密码进行哈希
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return err
	}

	// 插入新用户
	_, err = d.db.Exec(`
		INSERT INTO users (username, password)
		VALUES ($1, $2)
	`, username, hashedPassword)
	return err
}

func (d *Database) InsertDeal(deal *Deal) error {
	// Generate UUID if not provided
	if deal.UUID == "" {
		u, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		deal.UUID = u.String()
	}

	now := time.Now()
	if deal.CreatedAt.IsZero() {
		deal.CreatedAt = now
	}
	if deal.UpdatedAt.IsZero() {
		deal.UpdatedAt = now
	}

	err := d.db.QueryRow(`
		INSERT INTO deals (uuid, storage_provider, client_wallet, payload_cid, commp, start_epoch, end_epoch, provider_collateral, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING uuid, created_at, updated_at`,
		deal.UUID, deal.StorageProvider, deal.ClientWallet, deal.PayloadCid, deal.CommP,
		deal.StartEpoch, deal.EndEpoch, deal.ProviderCollateral, deal.Status,
		deal.CreatedAt, deal.UpdatedAt,
	).Scan(&deal.UUID, &deal.CreatedAt, &deal.UpdatedAt)

	return err
}

func (d *Database) GetDeal(uuid string) (*Deal, error) {
	deal := &Deal{}
	err := d.db.QueryRow(`
		SELECT uuid, storage_provider, client_wallet, payload_cid, commp, start_epoch, end_epoch, 
		       provider_collateral, status, created_at, updated_at
		FROM deals
		WHERE uuid = $1`,
		uuid,
	).Scan(
		&deal.UUID,
		&deal.StorageProvider,
		&deal.ClientWallet,
		&deal.PayloadCid,
		&deal.CommP,
		&deal.StartEpoch,
		&deal.EndEpoch,
		&deal.ProviderCollateral,
		&deal.Status,
		&deal.CreatedAt,
		&deal.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return deal, nil
}

func (d *Database) UpdateDealStatus(uuid string, status string) error {
	_, err := d.db.Exec(`
		UPDATE deals 
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE uuid = $2`,
		status, uuid,
	)
	return err
}

func (d *Database) ListDeals() ([]Deal, error) {
	rows, err := d.db.Query(`
		SELECT uuid, storage_provider, client_wallet, payload_cid, commp, start_epoch, end_epoch,
		       provider_collateral, status, created_at, updated_at
		FROM deals
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deals []Deal
	for rows.Next() {
		var deal Deal
		err := rows.Scan(
			&deal.UUID,
			&deal.StorageProvider,
			&deal.ClientWallet,
			&deal.PayloadCid,
			&deal.CommP,
			&deal.StartEpoch,
			&deal.EndEpoch,
			&deal.ProviderCollateral,
			&deal.Status,
			&deal.CreatedAt,
			&deal.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		deals = append(deals, deal)
	}
	return deals, nil
}

func (d *Database) GetDealsByStatus(status string) ([]Deal, error) {
	rows, err := d.db.Query(`
		SELECT uuid, storage_provider, client_wallet, payload_cid, commp, 
			   start_epoch, end_epoch, provider_collateral, status, 
			   created_at, updated_at
		FROM deals
		WHERE status = $1
		ORDER BY created_at ASC
	`, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query deals: %v", err)
	}
	defer rows.Close()

	var deals []Deal
	for rows.Next() {
		var deal Deal
		err := rows.Scan(
			&deal.UUID,
			&deal.StorageProvider,
			&deal.ClientWallet,
			&deal.PayloadCid,
			&deal.CommP,
			&deal.StartEpoch,
			&deal.EndEpoch,
			&deal.ProviderCollateral,
			&deal.Status,
			&deal.CreatedAt,
			&deal.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan deal: %v", err)
		}
		deals = append(deals, deal)
	}

	return deals, nil
}

func (d *Database) GetFilesByPieceCids(pieceCids []string) ([]CarFile, error) {
	query := `
		SELECT id, comm_p, data_cid, piece_cid, piece_size, car_size, 
			   file_path, raw_files, deal_status, deal_time, deal_error, 
			   deal_id, regenerate_status, created_at, updated_at
		FROM files
		WHERE piece_cid = ANY($1)
		ORDER BY created_at DESC
	`

	rows, err := d.db.Query(query, pq.Array(pieceCids))
	if err != nil {
		return nil, fmt.Errorf("failed to query files by piece CIDs: %v", err)
	}
	defer rows.Close()

	var files []CarFile
	for rows.Next() {
		var file CarFile
		err := rows.Scan(
			&file.ID,
			&file.CommP,
			&file.DataCid,
			&file.PieceCid,
			&file.PieceSize,
			&file.CarSize,
			&file.FilePath,
			&file.RawFiles,
			&file.DealStatus,
			&file.DealTime,
			&file.DealError,
			&file.DealID,
			&file.RegenerateStatus,
			&file.CreatedAt,
			&file.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file: %v", err)
		}
		files = append(files, file)
	}

	return files, nil
}

func (d *Database) ListPendingFiles() ([]CarFile, error) {
	query := `
		SELECT id, comm_p, data_cid, piece_cid, piece_size, car_size, 
			   file_path, raw_files, deal_status, deal_time, deal_error, 
			   deal_id, regenerate_status, created_at, updated_at
		FROM files
		WHERE deal_status = $1
		ORDER BY created_at DESC
	`

	rows, err := d.db.Query(query, DealStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending files: %v", err)
	}
	defer rows.Close()

	var files []CarFile
	for rows.Next() {
		var file CarFile
		err := rows.Scan(
			&file.ID,
			&file.CommP,
			&file.DataCid,
			&file.PieceCid,
			&file.PieceSize,
			&file.CarSize,
			&file.FilePath,
			&file.RawFiles,
			&file.DealStatus,
			&file.DealTime,
			&file.DealError,
			&file.DealID,
			&file.RegenerateStatus,
			&file.CreatedAt,
			&file.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file: %v", err)
		}
		files = append(files, file)
	}

	return files, nil
}

func (d *Database) GetFilesByDealStatus(status string, startTime, endTime *time.Time) ([]CarFile, error) {
	var query string
	var args []interface{}
	var argCount int = 1

	// Base query
	query = `
		SELECT id, comm_p, data_cid, piece_cid, piece_size, car_size, 
			   file_path, raw_files, deal_status, deal_time, deal_error, 
			   deal_id, regenerate_status, created_at, updated_at
		FROM files
		WHERE 1=1
	`

	// Add status filter if provided
	if status != "" {
		query += fmt.Sprintf(" AND deal_status = $%d", argCount)
		args = append(args, status)
		argCount++
	}

	// Add time range filters if provided
	if startTime != nil {
		query += fmt.Sprintf(" AND deal_time >= $%d", argCount)
		args = append(args, startTime)
		argCount++
	}
	if endTime != nil {
		query += fmt.Sprintf(" AND deal_time <= $%d", argCount)
		args = append(args, endTime)
		argCount++
	}

	// Add order by
	query += " ORDER BY deal_time DESC"

	// Execute query
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query files: %v", err)
	}
	defer rows.Close()

	var files []CarFile
	for rows.Next() {
		var file CarFile
		err := rows.Scan(
			&file.ID,
			&file.CommP,
			&file.DataCid,
			&file.PieceCid,
			&file.PieceSize,
			&file.CarSize,
			&file.FilePath,
			&file.RawFiles,
			&file.DealStatus,
			&file.DealTime,
			&file.DealError,
			&file.DealID,
			&file.RegenerateStatus,
			&file.CreatedAt,
			&file.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file: %v", err)
		}
		files = append(files, file)
	}

	return files, nil
}

func (d *Database) UpdateRegenerateStatus(id string, status RegenerateStatus) error {
	result, err := d.db.Exec(`
		UPDATE files 
		SET regenerate_status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`, status, id)
	if err != nil {
		return fmt.Errorf("failed to update regenerate status: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("file not found with id: %s", id)
	}

	return nil
}

func (d *Database) GetFileByCommP(commp string) (*CarFile, error) {
	var file CarFile
	var rawFiles sql.NullString
	var dealTime sql.NullTime
	var dealID sql.NullString
	var regenerateStatus sql.NullString

	query := `
		SELECT id, commp, data_cid, piece_cid, piece_size, car_size, file_path, 
		raw_files, deal_status, deal_time, deal_error, deal_id, regenerate_status
		FROM files WHERE commp = $1
	`

	err := d.db.QueryRow(query, commp).Scan(
		&file.ID,
		&file.CommP,
		&file.DataCid,
		&file.PieceCid,
		&file.PieceSize,
		&file.CarSize,
		&file.FilePath,
		&rawFiles,
		&file.DealStatus,
		&dealTime,
		&file.DealError,
		&dealID,
		&regenerateStatus,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("file with commp %s not found", commp)
	}
	if err != nil {
		return nil, fmt.Errorf("error querying file: %w", err)
	}

	// 处理可空字段
	if rawFiles.Valid {
		file.RawFiles = rawFiles.String
	}
	if dealTime.Valid {
		file.DealTime = &dealTime.Time
	}
	if dealID.Valid {
		file.DealID = &dealID.String
	}
	if regenerateStatus.Valid {
		file.RegenerateStatus = RegenerateStatus(regenerateStatus.String)
	}

	return &file, nil
}

// GetProposedDealsWithRegeneratedFiles 获取status为proposed且对应文件regenerate_status为success的订单
func (d *Database) GetProposedDealsWithRegeneratedFiles() ([]Deal, error) {
	query := `
		SELECT DISTINCT d.uuid, d.storage_provider, d.client_wallet, d.payload_cid, 
		d.commp, d.start_epoch, d.end_epoch, d.provider_collateral, d.status, 
		d.created_at, d.updated_at
		FROM deals d
		JOIN files f ON d.commp = f.comm_p
		WHERE d.status = 'proposed'
		AND f.regenerate_status = 'success'
	`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying deals: %w", err)
	}
	defer rows.Close()

	var deals []Deal
	for rows.Next() {
		var deal Deal
		err := rows.Scan(
			&deal.UUID,
			&deal.StorageProvider,
			&deal.ClientWallet,
			&deal.PayloadCid,
			&deal.CommP,
			&deal.StartEpoch,
			&deal.EndEpoch,
			&deal.ProviderCollateral,
			&deal.Status,
			&deal.CreatedAt,
			&deal.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning deal: %w", err)
		}
		deals = append(deals, deal)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating deals: %w", err)
	}

	return deals, nil
}

// GetSuccessDeals 获取所有状态为success的订单
func (d *Database) GetSuccessDeals() ([]Deal, error) {
	rows, err := d.db.Query(`
		SELECT uuid, commp, storage_provider, client_wallet, status, created_at, updated_at
		FROM deals
		WHERE status = 'success'
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deals []Deal
	for rows.Next() {
		var deal Deal
		err := rows.Scan(
			&deal.UUID,
			&deal.CommP,
			&deal.StorageProvider,
			&deal.ClientWallet,
			&deal.Status,
			&deal.CreatedAt,
			&deal.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		deals = append(deals, deal)
	}

	return deals, nil
}

// InitFromConfig 从配置文件初始化数据库连接
func InitFromConfig(cfg *config.Config) (*Database, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	dbConfig := &DBConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	}

	return InitDB(dbConfig)
}
