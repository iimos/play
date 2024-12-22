package store

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"os"
	"time"
	_ "time/tzdata"
)

var ClickhouseURL = "127.0.0.1:9000"

var TimezoneMSK *time.Location

func init() {
	tz, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		panic(err)
	}
	TimezoneMSK = tz

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
