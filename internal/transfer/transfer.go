// Package transfer imports a stopped writer's exact PostgreSQL18 CSV snapshot.
// It never renders messages or connects to SMTP.
package transfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var Columns = []string{"id", "client_id", "idempotency_digest", "request_digest", "recipient_digest", "template_id", "state", "attempt_count", "created_at", "updated_at", "delivered_at"}

func Validate(data []byte, expected string) (int, error) {
	digest := sha256.Sum256(data)
	if len(expected) != 64 || hex.EncodeToString(digest[:]) != expected {
		return 0, errors.New("source digest differs")
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = len(Columns)
	header, err := r.Read()
	if err != nil || !reflect.DeepEqual(header, Columns) {
		return 0, errors.New("source header differs")
	}
	count := 0
	last := ""
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, errors.New("source row rejected")
		}
		if row[0] <= last {
			return 0, errors.New("source IDs are not strictly ordered")
		}
		last = row[0]
		for _, i := range []int{2, 3, 4} {
			if !strings.HasPrefix(row[i], `\x`) || len(row[i]) != 66 {
				return 0, errors.New("source digest encoding rejected")
			}
			if _, err := hex.DecodeString(row[i][2:]); err != nil {
				return 0, errors.New("source digest encoding rejected")
			}
		}
		count++
	}
	return count, nil
}

func Protected(path string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > maximum {
		return nil, errors.New("protected input rejected")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("protected input unavailable")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maximum+1))
	if err != nil || int64(len(data)) > maximum {
		return nil, errors.New("protected input rejected")
	}
	return data, nil
}

// Apply refuses nonempty targets. Full COPY OUT comparison precedes COMMIT.
// verifyOnly is read-only and can safely resume after an uncertain import response.
func Apply(ctx context.Context, databaseURL string, data []byte, expected string, verifyOnly, dryRun bool) (int, error) {
	if verifyOnly && dryRun {
		return 0, errors.New("conflicting transfer modes")
	}
	count, err := Validate(data, expected)
	if err != nil {
		return 0, err
	}
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return 0, errors.New("target configuration rejected")
	}
	cfg := poolConfig.ConnConfig
	cfg.ConnectTimeout = 5 * time.Second
	if cfg.Database != "mail" || (cfg.User != "mail_migration" && !(verifyOnly && cfg.User == "mail_runtime")) {
		return 0, errors.New("target identity rejected")
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return 0, errors.New("target connection failed")
	}
	defer conn.Close(context.Background())
	options := pgx.TxOptions{IsoLevel: pgx.Serializable}
	if verifyOnly {
		options.AccessMode = pgx.ReadOnly
	}
	tx, err := conn.BeginTx(ctx, options)
	if err != nil {
		return 0, errors.New("target transaction failed")
	}
	defer tx.Rollback(context.Background())
	for _, sql := range []string{"SET LOCAL TIME ZONE 'UTC'", "SET LOCAL DateStyle = 'ISO, YMD'", "SET LOCAL bytea_output = 'hex'", "SET LOCAL lock_timeout = '5s'", "SET LOCAL statement_timeout = '30s'"} {
		if _, err := tx.Exec(ctx, sql); err != nil {
			return 0, errors.New("target session rejected")
		}
	}
	columns := strings.Join(Columns, ",")
	if !verifyOnly {
		if _, err := tx.Exec(ctx, "LOCK TABLE app.mail_deliveries IN ACCESS EXCLUSIVE MODE"); err != nil {
			return 0, errors.New("target lock failed")
		}
		var existing int64
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM app.mail_deliveries").Scan(&existing); err != nil || existing != 0 {
			return 0, errors.New("target is not empty; verify before any retry")
		}
		command, err := tx.Conn().PgConn().CopyFrom(ctx, bytes.NewReader(data), "COPY app.mail_deliveries ("+columns+") FROM STDIN WITH (FORMAT csv, HEADER true)")
		if err != nil || command.RowsAffected() != int64(count) {
			return 0, errors.New("target import failed")
		}
	}
	var actual bytes.Buffer
	if _, err := tx.Conn().PgConn().CopyTo(ctx, &actual, "COPY (SELECT "+columns+" FROM app.mail_deliveries ORDER BY id) TO STDOUT WITH (FORMAT csv, HEADER true)"); err != nil {
		return 0, errors.New("target comparison failed")
	}
	if _, err := Validate(actual.Bytes(), expected); err != nil || !bytes.Equal(actual.Bytes(), data) {
		return 0, errors.New("target differs; transaction not committed")
	}
	if dryRun {
		if err := tx.Rollback(ctx); err != nil {
			return 0, errors.New("dry-run rollback failed")
		}
		return count, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, errors.New("commit outcome uncertain; run verify-only")
	}
	return count, nil
}
