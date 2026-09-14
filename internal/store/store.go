package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

// Bin - корзина для входящих запросов
type Bin struct {
	ID        string
	CreatedAt time.Time
}

// Request - сохранённый HTTP-запрос
type Request struct {
	ID          int64
	BinID       string
	Method      string
	Path        string
	QueryString string
	Headers     map[string][]string
	Body        string
	RemoteAddr  string
	CreatedAt   time.Time
}

// Store работает с PostgreSQL через pgxpool
type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// ConnectWithRetry ждёт готовности Postgres и периодически повторяет подключение
func ConnectWithRetry(ctx context.Context, databaseURL string, attempts int, delay time.Duration) (*pgxpool.Pool, error) {
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for i := 1; i <= attempts; i++ {
		pool, err := Connect(ctx, databaseURL)
		if err == nil {
			return pool, nil
		}
		lastErr = err

		if i == attempts {
			break
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, fmt.Errorf("connect database after %d attempts: %w", attempts, lastErr)
}

func (s *Store) CreateBin(ctx context.Context, id string) (Bin, error) {
	const q = `
		INSERT INTO bins (id)
		VALUES ($1)
		RETURNING id, created_at
	`

	var bin Bin
	err := s.pool.QueryRow(ctx, q, id).Scan(&bin.ID, &bin.CreatedAt)
	if err != nil {
		return Bin{}, fmt.Errorf("create bin: %w", err)
	}
	return bin, nil
}

func (s *Store) GetBin(ctx context.Context, id string) (Bin, error) {
	const q = `
		SELECT id, created_at
		FROM bins
		WHERE id = $1
	`

	var bin Bin
	err := s.pool.QueryRow(ctx, q, id).Scan(&bin.ID, &bin.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Bin{}, ErrNotFound
	}
	if err != nil {
		return Bin{}, fmt.Errorf("get bin: %w", err)
	}
	return bin, nil
}

func (s *Store) SaveRequest(ctx context.Context, req Request) (Request, error) {
	headersJSON, err := json.Marshal(req.Headers)
	if err != nil {
		return Request{}, fmt.Errorf("marshal headers: %w", err)
	}

	const q = `
		INSERT INTO requests (bin_id, method, path, query_string, headers, body, remote_addr)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	err = s.pool.QueryRow(
		ctx,
		q,
		req.BinID,
		req.Method,
		req.Path,
		req.QueryString,
		headersJSON,
		req.Body,
		req.RemoteAddr,
	).Scan(&req.ID, &req.CreatedAt)
	if err != nil {
		return Request{}, fmt.Errorf("save request: %w", err)
	}
	return req, nil
}

func (s *Store) ListRequests(ctx context.Context, binID string, limit int) ([]Request, error) {
	if limit <= 0 {
		limit = 50
	}

	const q = `
		SELECT id, bin_id, method, path, query_string, headers, body, remote_addr, created_at
		FROM requests
		WHERE bin_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`

	rows, err := s.pool.Query(ctx, q, binID, limit)
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}
	defer rows.Close()

	var result []Request
	for rows.Next() {
		var (
			req         Request
			headersJSON []byte
		)
		if err := rows.Scan(
			&req.ID,
			&req.BinID,
			&req.Method,
			&req.Path,
			&req.QueryString,
			&headersJSON,
			&req.Body,
			&req.RemoteAddr,
			&req.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan request: %w", err)
		}

		if err := json.Unmarshal(headersJSON, &req.Headers); err != nil {
			return nil, fmt.Errorf("unmarshal headers: %w", err)
		}
		if req.Headers == nil {
			req.Headers = map[string][]string{}
		}

		result = append(result, req)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate requests: %w", err)
	}

	return result, nil
}
