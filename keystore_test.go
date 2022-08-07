package pgks

import (
	"context"
	"crypto/rand"
	"fmt"
	keystore "github.com/ipfs/go-ipfs-keystore"
	"github.com/jackc/pgx/v4"
	ci "github.com/libp2p/go-libp2p-core/crypto"
	"os"
	"sort"
	"sync"
	"testing"
)

var initOnce sync.Once

func envString(t *testing.T, key string, defaultValue string) string {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	return v
}

type rr struct{}

func (rr rr) Read(b []byte) (int, error) {
	return rand.Read(b)
}

func privKeyOrFatal(t *testing.T) ci.PrivKey {
	priv, _, err := ci.GenerateEd25519Key(rr{})
	if err != nil {
		t.Fatal(err)
	}
	return priv
}

// Automatically re-create the test datastore.
func initPG(t *testing.T) {
	initOnce.Do(func() {
		connConf, err := pgx.ParseConfig(fmt.Sprintf(
			"postgres://%s:%s@%s/%s?sslmode=disable",
			envString(t, "PG_USER", "postgres"),
			envString(t, "PG_PASS", "123456"),
			envString(t, "PG_HOST", "127.0.0.1"),
			envString(t, "PG_DB", envString(t, "PG_USER", "postgres")),
		))
		if err != nil {
			t.Fatal(err)
		}
		conn, err := pgx.ConnectConfig(context.Background(), connConf)
		if err != nil {
			t.Fatal(err)
		}
		_, err = conn.Exec(context.Background(), "DROP DATABASE IF EXISTS test_keystore")
		if err != nil {
			t.Fatal(err)
		}
		_, err = conn.Exec(context.Background(), "CREATE DATABASE test_keystore")
		if err != nil {
			t.Fatal(err)
		}
		err = conn.Close(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	})
}

// returns datastore, and a function to call on exit.
//
//  d, close := newDS(t)
//  defer close()
func newKS(t *testing.T) (keystore.Keystore, func()) {
	initPG(t)
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=disable",
		envString(t, "PG_USER", "postgres"),
		envString(t, "PG_PASS", "123456"),
		envString(t, "PG_HOST", "127.0.0.1"),
		"test_keystore",
	)
	connConf, err := pgx.ParseConfig(connString)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.ConnectConfig(context.Background(), connConf)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS keys (name TEXT NOT NULL PRIMARY KEY, data BYTEA)")
	if err != nil {
		t.Fatal(err)
	}
	d, err := NewKeystore(context.Background(), connString)
	return d, func() {
		_, _ = conn.Exec(context.Background(), "DROP TABLE IF EXISTS keys")
		_ = conn.Close(context.Background())
	}
}

func TestKeystoreBasics(t *testing.T) {
	ks, done := newKS(t)
	defer done()

	l, err := ks.List()
	if err != nil {
		t.Fatal(err)
	}

	if len(l) != 0 {
		t.Fatal("expected no keys")
	}

	k1 := privKeyOrFatal(t)
	k2 := privKeyOrFatal(t)
	k3 := privKeyOrFatal(t)
	k4 := privKeyOrFatal(t)

	err = ks.Put("foo", k1)
	if err != nil {
		t.Fatal(err)
	}

	err = ks.Put("bar", k2)
	if err != nil {
		t.Fatal(err)
	}

	l, err = ks.List()
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(l)
	if len(l) < 2 {
		t.Fatal("wrong entries listed")
	}
	if l[0] != "bar" || l[1] != "foo" {
		t.Fatal("wrong entries listed")
	}

	err = ks.Put("foo", k3)
	if err == nil {
		t.Fatal("should not be able to overwrite key")
	}

	exist, err := ks.Has("foo")
	if !exist {
		t.Fatal("should know it has a key named foo")
	}
	if err != nil {
		t.Fatal(err)
	}

	exist, err = ks.Has("nonexistingkey")
	if exist {
		t.Fatal("should know it doesn't have a key named nonexistingkey")
	}
	if err != nil {
		t.Fatal(err)
	}

	if err := ks.Delete("bar"); err != nil {
		t.Fatal(err)
	}

	if err := ks.Put("beep", k3); err != nil {
		t.Fatal(err)
	}

	if err := ks.Put("boop", k4); err != nil {
		t.Fatal(err)
	}

	if err := assertGetKey(ks, "foo", k1); err != nil {
		t.Fatal(err)
	}

	if err := assertGetKey(ks, "beep", k3); err != nil {
		t.Fatal(err)
	}

	if err := assertGetKey(ks, "boop", k4); err != nil {
		t.Fatal(err)
	}

	if err := ks.Put("..///foo/", k1); err != nil {
		t.Fatal(err)
	}

	if err := ks.Put("", k1); err == nil {
		t.Fatal("shouldn't be able to put a key with no name")
	}

	if err := ks.Put(".foo", k1); err != nil {
		t.Fatal(err)
	}
}

func assertGetKey(ks keystore.Keystore, name string, exp ci.PrivKey) error {
	outK, err := ks.Get(name)
	if err != nil {
		return err
	}

	if !outK.Equals(exp) {
		return fmt.Errorf("key we got out didn't match expectation")
	}

	return nil
}
