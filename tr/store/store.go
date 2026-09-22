package store

import (
	"context"
	"os"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

var ClickhouseURL = "127.0.0.1:9000"

func init() {
	if u := os.Getenv("CLICKHOUSE_URL"); u != "" {
		ClickhouseURL = u
	}
}

type Store struct {
	conn driver.Conn
}

func New() (*Store, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{ClickhouseURL},
		Auth: clickhouse.Auth{
			Database: "tr",
		},
		Settings: clickhouse.Settings{"max_execution_time": 60},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout:          30 * time.Second,
		MaxOpenConns:         5,
		MaxIdleConns:         5,
		ConnMaxLifetime:      time.Duration(10) * time.Minute,
		ConnOpenStrategy:     clickhouse.ConnOpenInOrder,
		BlockBufferSize:      10,
		MaxCompressionBuffer: 10240,
		ClientInfo: clickhouse.ClientInfo{ // optional, please see Client info section in the README.md
			Products: []struct {
				Name    string
				Version string
			}{
				{Name: "tr", Version: "0.1"},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return &Store{conn: conn}, nil
}

func (s *Store) Close() error {
	return s.conn.Close()
}

func coalesce[T any](x *T) *T {
	if x == nil {
		var empty T
		return &empty
	}
	return x
}

// lastDate выполняет запрос, возвращающий одну дату, и нормализует пустой
// результат (ClickHouse отдаёт 1970-01-01 вместо NULL).
func (s *Store) lastDate(ctx context.Context, query string, args ...any) (time.Time, error) {
	var d time.Time
	if err := s.conn.QueryRow(ctx, query, args...).Scan(&d); err != nil {
		return time.Time{}, err
	}
	if d.Year() < 2000 {
		return time.Time{}, nil
	}
	return d, nil
}
