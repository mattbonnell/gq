package test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/mattbonnell/gq"
	"github.com/ory/dockertest/v3"
)

var pool *dockertest.Pool
var postgres *dockertest.Resource
var mysql *dockertest.Resource

func TestMain(m *testing.M) {
	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	var err error
	log.Println("creating new dockertest pool")
	pool, err = dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	postgres = setupResource(pool, "postgres", "latest", []string{"POSTGRES_PASSWORD=secret", "POSTGRES_DB=gq"})
	mysql = setupResource(pool, "mysql", "latest", []string{"MYSQL_ROOT_PASSWORD=secret", "MYSQL_DATABASE=gq"})

	code := m.Run()

	teardownResource(pool, postgres)
	teardownResource(pool, mysql)

	os.Exit(code)
}

func TestPostgres(t *testing.T) {
	db, err := connectToDB(pool, postgres, fmt.Sprintf("postgres://postgres:secret@localhost:%s/gq?sslmode=disable", postgres.GetPort("5432/tcp")), "postgres")
	if err != nil {
		t.Fatalf("Couldn't connect to DB: %s\n", err)
	}
	client, err := gq.NewClient(db, "postgres")
	if err != nil {
		t.Fatalf("Could not create client: %s", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan interface{})

	if _, err := client.NewConsumer(ctx, func(m []byte) error {
		done <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("Could not create consumer: %s", err)
	}

	p, err := client.NewProducer(ctx)
	if err != nil {
		t.Fatalf("Could not create producer: %s", err)
	}

	t.Run("push and receive one message", func(t *testing.T) {
		p.Push([]byte("dummy"))
		select {
		case <-time.NewTimer(time.Second).C:
			t.Fatal("Timed-out waiting to receive message")
		case <-done:
			t.Log("message received")
		}
	})

}

func TestMySQL(t *testing.T) {
	db, err := connectToDB(pool, mysql, fmt.Sprintf("root:secret@(localhost:%s)/gq?parseTime=true", mysql.GetPort("3306/tcp")), "mysql")
	if err != nil {
		t.Fatalf("Couldn't connect to DB: %s\n", err)
	}
	client, err := gq.NewClient(db, "mysql")
	if err != nil {
		t.Fatalf("Could not create client: %s", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan interface{})

	if _, err := client.NewConsumer(ctx, func(m []byte) error {
		done <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("Could not create consumer: %s", err)
	}

	p, err := client.NewProducer(ctx)
	if err != nil {
		t.Fatalf("Could not create producer: %s", err)
	}

	t.Run("push and receive one message", func(t *testing.T) {
		p.Push([]byte("dummy"))
		select {
		case <-time.NewTimer(time.Second).C:
			t.Fatal("Timed-out waiting to receive message")
		case <-done:
			t.Log("message received")
		}
	})

}

func setupResourceWithRunOptions(pool *dockertest.Pool, opts *dockertest.RunOptions) *dockertest.Resource {
	// pulls an image, creates a container based on it and runs it
	log.Printf("spinning up %s:%s\n", opts.Repository, opts.Tag)
	var resource *dockertest.Resource
	if opts != nil {

	}
	resource, err := pool.RunWithOptions(opts)
	if err != nil {
		log.Fatalf("Could not start %s:%s: %s", opts.Repository, opts.Tag, err)
	}
	return resource

}

func setupResource(pool *dockertest.Pool, repository string, tag string, env []string) *dockertest.Resource {
	return setupResourceWithRunOptions(pool, &dockertest.RunOptions{Repository: repository, Tag: tag, Env: env})
}

func teardownResource(pool *dockertest.Pool, resource *dockertest.Resource) {
	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not tear-down resource: %s", err)
	}
}

func connectToDB(pool *dockertest.Pool, resource *dockertest.Resource, dsn string, driverName string) (*sql.DB, error) {
	// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
	var db *sql.DB
	if err := pool.Retry(func() error {
		var err error
		db, err = sql.Open(driverName, dsn)
		if err != nil {
			return err
		}
		return db.Ping()
	}); err != nil {
		return nil, fmt.Errorf("Could not connect to docker: %s", err)
	}
	return db, nil
}
