package delivery

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// VerifySchema performs no DDL or delivery writes. Missing columns, types,
// nullability or the indexes needed for deduplication fail before HTTP startup.
func (s *Store) VerifySchema(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return errors.New("begin read-only mail schema verification")
	}
	defer tx.Rollback(ctx)
	var valid bool
	err = tx.QueryRow(ctx, `
		WITH required(name, type, nullable) AS (VALUES
			('id','text',false), ('client_id','text',false),
			('idempotency_digest','bytea',false), ('request_digest','bytea',false),
			('recipient_digest','bytea',false), ('template_id','text',false),
			('state','text',false), ('attempt_count','integer',false),
			('created_at','timestamp with time zone',false),
			('updated_at','timestamp with time zone',false),
			('delivered_at','timestamp with time zone',true)
		), columns_valid AS (
			SELECT NOT EXISTS (SELECT 1 FROM required r WHERE NOT EXISTS (
				SELECT 1 FROM pg_catalog.pg_attribute a
				WHERE a.attrelid=to_regclass('public.mail_deliveries')
				AND a.attname=r.name AND NOT a.attisdropped
				AND a.atttypid::regtype::text=r.type AND a.attnotnull=(NOT r.nullable)
			)) AS valid
		), indexes AS (
			SELECT i.indisunique, i.indisprimary,
				ARRAY(SELECT a.attname::text FROM unnest(i.indkey) WITH ORDINALITY k(n, pos)
				JOIN pg_catalog.pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.n
				WHERE k.pos <= i.indnkeyatts ORDER BY k.pos) AS names
			FROM pg_catalog.pg_index i WHERE i.indrelid=to_regclass('public.mail_deliveries')
			AND i.indisvalid AND i.indisready AND i.indimmediate
			AND i.indpred IS NULL AND i.indexprs IS NULL
		)
		SELECT (SELECT valid FROM columns_valid)
		AND EXISTS (SELECT 1 FROM indexes WHERE indisprimary AND names=ARRAY['id'])
		AND EXISTS (SELECT 1 FROM indexes WHERE indisunique AND names=ARRAY['client_id','idempotency_digest'])
		AND EXISTS (SELECT 1 FROM indexes WHERE names=ARRAY['updated_at'])
		AND EXISTS (SELECT 1 FROM pg_catalog.pg_attrdef d JOIN pg_catalog.pg_attribute a
			ON a.attrelid=d.adrelid AND a.attnum=d.adnum
			WHERE d.adrelid=to_regclass('public.mail_deliveries') AND a.attname='attempt_count'
			AND pg_get_expr(d.adbin,d.adrelid)='1')
	`).Scan(&valid)
	if err != nil || !valid {
		return errors.New("mail database schema is unavailable or incompatible")
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.New("finish read-only mail schema verification")
	}
	return nil
}
