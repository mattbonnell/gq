package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/mattbonnell/gq"
	"github.com/ory/dockertest/v3"
)

func main() {
	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	log.Println("creating new dockertest pool")
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	go TestPostgres(pool, &wg)
	go TestMySQL(pool, &wg)
	wg.Wait()
}

func TestPostgres(pool *dockertest.Pool, wg *sync.WaitGroup) {
	defer wg.Done()
	postgres := setupResource(pool, "postgres", "latest", []string{"POSTGRES_PASSWORD=secret", "POSTGRES_DB=gq"})
	defer teardownResource(pool, postgres)
	db := connectToDB(pool, postgres, fmt.Sprintf("postgres://postgres:secret@localhost:%s/gq?sslmode=disable", postgres.GetPort("5432/tcp")), "postgres")
	defer db.Close()
	cl, err := gq.NewClient(db, "postgres")
	if err != nil {
		log.Fatalf("could not create client: %s", err)
	}
	if err := runTests(cl); err != nil {
		log.Fatalf("postgres: %s\n", err)
	}
}

func TestMySQL(pool *dockertest.Pool, wg *sync.WaitGroup) {
	defer wg.Done()
	mysql := setupResource(pool, "mysql", "latest", []string{"MYSQL_ROOT_PASSWORD=secret", "MYSQL_DATABASE=gq"})
	defer teardownResource(pool, mysql)
	db := connectToDB(pool, mysql, fmt.Sprintf("root:secret@(localhost:%s)/gq?parseTime=true", mysql.GetPort("3306/tcp")), "mysql")
	defer db.Close()
	cl, err := gq.NewClient(db, "mysql")
	if err != nil {
		log.Fatalf("could not create client: %s", err)
	}
	if err := runTests(cl); err != nil {
		log.Fatalf("mysql: %s\n", err)
	}
}

func runTests(cl *gq.Client) error {
	if err := pushAndReceiveOneMessage(cl); err != nil {
		return fmt.Errorf("pushAndReceiveOneMessage failed: %s", err)
	}
	if err := pushAndReceiveThreeMessages(cl); err != nil {
		return fmt.Errorf("pushAndReceiveThreeMessages failed: %s", err)
	}
	return nil
}

func pushAndReceiveOneMessage(cl *gq.Client) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan interface{})

	if _, err := cl.NewConsumer(ctx, func(m []byte) error {
		done <- struct{}{}
		return nil
	}); err != nil {
		return fmt.Errorf("could not create consumer: %s", err)
	}

	p, err := cl.NewProducer(ctx)
	if err != nil {
		return fmt.Errorf("could not create producer: %s", err)
	}

	p.Push([]byte("dummy"))
	select {
	case <-time.NewTimer(time.Second).C:
		return errors.New("timed-out waiting to receive message")
	case <-done:
		log.Println("message received")
		return nil
	}
}

func pushAndReceiveThreeMessages(cl *gq.Client) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan interface{})
	errChan := make(chan error)

	seen := make(map[string]bool)
	mu := sync.Mutex{}
	if _, err := cl.NewConsumer(ctx, func(m []byte) error {
		mu.Lock()
		idx := string(m)
		mu.Unlock()
		if _, ok := seen[idx]; ok {
			errChan <- fmt.Errorf("message received twice: %s", idx)
			return nil
		}
		if len(seen) == 3 {
			done <- struct{}{}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("could not create consumer: %s", err)
	}

	p, err := cl.NewProducer(ctx)
	if err != nil {
		return fmt.Errorf("could not create producer: %s", err)
	}

	p.Push([]byte("1"))
	p.Push([]byte("2"))
	p.Push([]byte("3"))
	select {
	case <-time.NewTimer(time.Second).C:
		return errors.New("timed-out waiting to receive message")
	case e := <-errChan:
		return e
	case <-done:
		log.Println("message received")
		return nil
	}
}

func setupResourceWithRunOptions(pool *dockertest.Pool, opts dockertest.RunOptions) *dockertest.Resource {
	// pulls an image, creates a container based on it and runs it
	log.Printf("spinning up %s:%s\n", opts.Repository, opts.Tag)
	if opts.Name == "" {
		opts.Name = opts.Repository
	}
	var resource *dockertest.Resource
	resource, err := pool.RunWithOptions(&opts)
	if err != nil {
		log.Fatalf("Could not start %s:%s: %s", opts.Repository, opts.Tag, err)
	}
	return resource

}

func setupResource(pool *dockertest.Pool, repository string, tag string, env []string) *dockertest.Resource {
	return setupResourceWithRunOptions(pool, dockertest.RunOptions{Repository: repository, Tag: tag, Env: env})
}

func teardownResource(pool *dockertest.Pool, resource *dockertest.Resource) {
	// You can't defer this because os.Exit doesn't care for defer
	log.Printf("tearing down %s\n", resource.Container.Name)
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not tear-down resource: %s", err)
	}
}

func connectToDB(pool *dockertest.Pool, resource *dockertest.Resource, dsn string, driverName string) *sql.DB {
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
		log.Fatalf("could not connect to docker: %s", err)
	}
	return db
}
