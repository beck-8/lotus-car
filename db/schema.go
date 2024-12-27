package db

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/google/uuid"
	"time"
)

type RawFileInfo struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	RelativePath string `json:"relative_path"`
}

// DealStatus 表示订单发送状态
type DealStatus string

const (
	DealStatusPending DealStatus = "pending"   // 未发送或正在发送
	DealStatusSuccess DealStatus = "success"   // 发送成功
	DealStatusFailed  DealStatus = "failed"    // 发送失败
)

type CarFile struct {
	ID          string     `json:"id"`
	CommP       string     `json:"commp"`
	DataCid     string     `json:"data_cid"`
	PieceCid    string     `json:"piece_cid"`
	PieceSize   uint64     `json:"piece_size"`
	CarSize     uint64     `json:"car_size"`
	FilePath    string     `json:"file_path"`
	RawFiles    string     `json:"raw_files"`  // JSON string of []RawFileInfo
	DealStatus  DealStatus `json:"deal_status"`
	DealTime    *time.Time `json:"deal_time"`    // 发单时间
	DealError   string     `json:"deal_error"`   // 发单失败的错误信息
	CreatedAt   time.Time  `json:"created_at"`
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

	// Create car_files table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS car_files (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
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
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create car_files table: %v", err)
	}

	return &Database{db: db}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) InsertCarFile(car *CarFile) error {
	// Generate UUID if not provided
	if car.ID == "" {
		u, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		car.ID = u.String()
	}

	err := d.db.QueryRow(`
		INSERT INTO car_files (id, comm_p, data_cid, piece_cid, piece_size, car_size, file_path, raw_files, deal_status, deal_time, deal_error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at`,
		car.ID, car.CommP, car.DataCid, car.PieceCid, car.PieceSize, car.CarSize, car.FilePath, car.RawFiles, car.DealStatus, car.DealTime, car.DealError,
	).Scan(&car.ID, &car.CreatedAt)

	return err
}

func (d *Database) ListCarFiles() ([]CarFile, error) {
	rows, err := d.db.Query(`
		SELECT id, comm_p, data_cid, piece_cid, piece_size, car_size, file_path, raw_files, deal_status, deal_time, deal_error, created_at
		FROM car_files
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var carFiles []CarFile
	for rows.Next() {
		var car CarFile
		err := rows.Scan(
			&car.ID,
			&car.CommP,
			&car.DataCid,
			&car.PieceCid,
			&car.PieceSize,
			&car.CarSize,
			&car.FilePath,
			&car.RawFiles,
			&car.DealStatus,
			&car.DealTime,
			&car.DealError,
			&car.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		carFiles = append(carFiles, car)
	}
	return carFiles, rows.Err()
}

func (d *Database) GetCarFile(id string) (*CarFile, error) {
	car := &CarFile{}
	err := d.db.QueryRow(`
		SELECT id, comm_p, data_cid, piece_cid, piece_size, car_size, file_path, raw_files, deal_status, deal_time, deal_error, created_at
		FROM car_files
		WHERE id = $1
	`, id).Scan(
		&car.ID,
		&car.CommP,
		&car.DataCid,
		&car.PieceCid,
		&car.PieceSize,
		&car.CarSize,
		&car.FilePath,
		&car.RawFiles,
		&car.DealStatus,
		&car.DealTime,
		&car.DealError,
		&car.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("car file with id %s not found", id)
	}
	if err != nil {
		return nil, err
	}
	return car, nil
}

func (d *Database) DeleteCarFile(id string) error {
	result, err := d.db.Exec("DELETE FROM car_files WHERE id = $1", id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("car file with id %s not found", id)
	}

	return nil
}

func (d *Database) SearchCarFiles(params SearchParams) ([]CarFile, error) {
	query := `
		SELECT id, comm_p, data_cid, piece_cid, piece_size, car_size, file_path, raw_files, deal_status, deal_time, deal_error, created_at 
		FROM car_files 
		WHERE 1=1
	`
	var args []interface{}
	paramCount := 1

	if params.CommP != "" {
		query += fmt.Sprintf(" AND comm_p = $%d", paramCount)
		args = append(args, params.CommP)
		paramCount++
	}
	if params.DataCid != "" {
		query += fmt.Sprintf(" AND data_cid = $%d", paramCount)
		args = append(args, params.DataCid)
		paramCount++
	}
	if params.PieceCid != "" {
		query += fmt.Sprintf(" AND piece_cid = $%d", paramCount)
		args = append(args, params.PieceCid)
		paramCount++
	}

	query += " ORDER BY created_at DESC"

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
			&file.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

func (d *Database) DB() *sql.DB {
	return d.db
}

func (d *Database) UpdateDealStatus(id string, status DealStatus, dealError string) error {
	now := time.Now()
	result, err := d.db.Exec(`
		UPDATE car_files 
		SET deal_status = $1,
		    deal_time = $2,
		    deal_error = $3
		WHERE id = $4`,
		status, now, dealError, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update deal status: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("car file with id %s not found", id)
	}

	return nil
}
