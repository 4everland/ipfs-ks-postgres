package pgks

import (
	"context"
	"errors"
	"fmt"
	ks "github.com/ipfs/go-ipfs-keystore"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	ci "github.com/libp2p/go-libp2p-core/crypto"
)

// PGKeystore is a PostgreSQL backed keystore.
type PGKeystore struct {
	table string
	pool  *pgxpool.Pool
}

// NewKeystore creates a new PostgreSQL datastore
func NewKeystore(ctx context.Context, connString string, options ...Option) (*PGKeystore, error) {
	cfg := Options{}
	err := cfg.Apply(append([]Option{OptionDefaults}, options...)...)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.Connect(ctx, connString)
	if err != nil {
		return nil, err
	}

	return &PGKeystore{table: cfg.Table, pool: pool}, nil
}

// PgxPool exposes the underlying pool of connections to Postgres.
func (k *PGKeystore) PgxPool() *pgxpool.Pool {
	return k.pool
}

// Close closes the underying PostgreSQL database.
func (k *PGKeystore) Close() error {
	if k.pool != nil {
		k.pool.Close()
	}
	return nil
}

func (k *PGKeystore) Has(name string) (bool, error) {
	sql := fmt.Sprintf("SELECT exists(SELECT 1 FROM %s WHERE name = $1)", k.table)
	row := k.pool.QueryRow(context.Background(), sql, name)
	var exists bool
	switch err := row.Scan(&exists); err {
	case pgx.ErrNoRows:
		return exists, ks.ErrNoSuchKey
	case nil:
		return exists, nil
	default:
		return exists, err
	}
}

// Put stores a key in the Keystore, if a key with the same name already exists, returns ErrKeyExists
func (k *PGKeystore) Put(name string, priv ci.PrivKey) error {
	if name == "" {
		return fmt.Errorf("key name must be at least one character")
	}
	exists, err := k.Has(name)
	if exists {
		return ks.ErrKeyExists
	}
	if err != nil && !errors.Is(err, ks.ErrNoSuchKey) {
		return err
	}
	skbytes, err := ci.MarshalPrivateKey(priv)
	if err != nil {
		return err
	}
	sql := fmt.Sprintf("INSERT INTO %s (name, data) VALUES ($1, $2) ON CONFLICT (name) DO UPDATE SET data = $2", k.table)
	_, err = k.pool.Exec(context.Background(), sql, name, skbytes)
	if err != nil {
		return err
	}
	return nil
}

// Get retrieves a key from the Keystore if it exists, and returns ErrNoSuchKey
// otherwise.
func (k *PGKeystore) Get(name string) (ci.PrivKey, error) {
	sql := fmt.Sprintf("SELECT data FROM %s WHERE name = $1", k.table)
	row := k.pool.QueryRow(context.Background(), sql, name)
	var out []byte
	switch err := row.Scan(&out); err {
	case pgx.ErrNoRows:
		return nil, ks.ErrNoSuchKey
	case nil:
		return ci.UnmarshalPrivateKey(out)
	default:
		return nil, err
	}
}

// Delete removes a key from the Keystore
func (k *PGKeystore) Delete(name string) error {
	sql := fmt.Sprintf("DELETE FROM %s WHERE name = $1", k.table)
	_, err := k.pool.Exec(context.Background(), sql, name)
	if err != nil {
		return err
	}
	return nil
}

// List returns a list of key identifier
func (k *PGKeystore) List() ([]string, error) {
	sql := fmt.Sprintf("SELECT name FROM %s", k.table)
	rows, err := k.pool.Query(context.Background(), sql)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0)
	for rows.Next() {
		var name string
		e := rows.Scan(&name)
		if e != nil {
			continue
		}
		names = append(names, name)
	}

	return names, nil
}
