package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/mattbonnell/gq"
	"github.com/ory/dockertest/v3"
	"github.com/rs/zerolog/log"
)

func main() {
	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	log.Info().Msg("creating new dockertest pool")
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatal().Msgf("could not connect to docker: %s", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	TestPostgres(pool, &wg)
	TestMySQL(pool, &wg)
	wg.Wait()
}

func TestPostgres(pool *dockertest.Pool, wg *sync.WaitGroup) {
	defer wg.Done()
	postgres, err := setupResource(pool, "postgres", "latest", "postgres", []string{"POSTGRES_PASSWORD=secret", "POSTGRES_DB=gq"})
	if err != nil {
		log.Error().Err(err).Msg("error occurred")
		return
	}
	defer teardownResource(pool, postgres)
	db, err := connectToDB(pool, postgres, fmt.Sprintf("postgres://postgres:secret@localhost:%s/gq?sslmode=disable", postgres.GetPort("5432/tcp")), "postgres")
	if err != nil {
		log.Error().Err(err).Msg("error occurred")
		return
	}
	defer db.Close()
	cl, err := gq.NewClient(db, "postgres")
	if err != nil {
		log.Error().Msgf("could not create client: %s", err)
		return
	}
	if err := runTests(cl, "postgres"); err != nil {
		log.Error().Msgf("postgres: %s", err)
		return
	}
	log.Info().Msg("postgres tests passed")
}

func TestMySQL(pool *dockertest.Pool, wg *sync.WaitGroup) {
	defer wg.Done()
	mysql, err := setupResource(pool, "mysql", "latest", "mysql", []string{"MYSQL_ROOT_PASSWORD=secret", "MYSQL_DATABASE=gq"})
	if err != nil {
		log.Error().Err(err).Msg("error occurred")
		return
	}
	defer teardownResource(pool, mysql)
	db, err := connectToDB(pool, mysql, fmt.Sprintf("root:secret@(localhost:%s)/gq?parseTime=true", mysql.GetPort("3306/tcp")), "mysql")
	if err != nil {
		log.Error().Err(err).Msg("error occurred")
		return
	}
	defer db.Close()
	cl, err := gq.NewClient(db, "mysql")
	if err != nil {
		log.Error().Msgf("could not create client: %s", err)
		return
	}
	if err := runTests(cl, "mysql"); err != nil {
		log.Error().Msgf("mysql: %s", err)
		return
	}
	log.Info().Msg("mysql tests passed")
}

func TestCockroach(pool *dockertest.Pool, wg *sync.WaitGroup) {
	defer wg.Done()
	crdb, err := setupResourceWithRunOptions(pool, dockertest.RunOptions{Name: "crdb", Repository: "cockroachdb/cockroach", Tag: "latest", Cmd: []string{"start-single-node", "--insecure"}})
	if err != nil {
		log.Error().Err(err).Msg("error occurred")
		return
	}
	defer teardownResource(pool, crdb)
	crdbInit, err := setupResource(
		pool,
		"timveil/cockroachdb-remote-client",
		"latest",
		"crdbinit",
		[]string{
			fmt.Sprintf("COCKROACH_HOST=%s:%s", crdb.Container.Name, crdb.GetPort("26267/tcp")),
			"DATABASE_NAME=gq",
			"COCKROACH_INSECURE=true",
			"COCKROACH_USER=root",
		},
	)
	if err != nil {
		log.Error().Err(err).Msg("error occurred")
		return
	}
	defer teardownResource(pool, crdbInit)
	db, err := connectToDB(pool, crdb, fmt.Sprintf("postgres://root@localhost:%s/gq?sslmode=disable", crdb.GetPort("26257/tcp")), "postgres")
	if err != nil {
		log.Error().Err(err).Msg("error occurred")
		return
	}
	defer db.Close()
	cl, err := gq.NewClient(db, "postgres")
	if err != nil {
		log.Error().Msgf("could not create client: %s", err)
		return
	}
	if err := runTests(cl, "cockroach"); err != nil {
		log.Error().Msgf("cockroach: %s", err)
		return
	}
	log.Info().Msg("cockroach tests passed")
}

func runTests(cl *gq.Client, label string) error {
	if err := pushAndReceiveNMessages(cl, 1, label); err != nil {
		return fmt.Errorf("pushAndReceiveOneMessage failed: %s", err)
	}
	log.Info().Msgf("%s: pushAndReceiveOneMessage passed", label)
	if err := pushAndReceiveNMessages(cl, 3, label); err != nil {
		return fmt.Errorf("pushAndReceiveThreeMessages failed: %s", err)
	}
	log.Info().Msgf("%s: pushAndReceiveThreeMessages passed", label)
	if err := pushAndReceiveNMessages(cl, 50, label); err != nil {
		return fmt.Errorf("pushAndReceiveFiftyMessages failed: %s", err)
	}
	log.Info().Msgf("%s: pushAndReceiveFiftyMessages passed", label)
	return nil
}

func pushAndReceiveNMessages(cl *gq.Client, n int, label string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan interface{})
	errChan := make(chan error)

	seen := make(map[string]bool)
	mu := sync.Mutex{}
	if _, err := cl.NewConsumer(ctx, func(m []byte) error {
		idx := string(m)
		log.Debug().Msgf("%s: received message %s", label, idx)
		mu.Lock()
		defer mu.Unlock()
		if _, ok := seen[idx]; ok {
			errChan <- fmt.Errorf("%s: message received twice: %s", label, idx)
			return nil
		}
		seen[idx] = true
		if len(seen) == n {
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

	for i := 1; i <= n; i++ {
		p.Push([]byte(fmt.Sprint(i)))
	}
	select {
	case <-time.NewTimer(time.Second * 30).C:
		return errors.New("timed-out waiting to receive message")
	case e := <-errChan:
		return e
	case <-done:
		return nil
	}
}

func setupResourceWithRunOptions(pool *dockertest.Pool, opts dockertest.RunOptions) (*dockertest.Resource, error) {
	// pulls an image, creates a container based on it and runs it
	log.Info().Msgf("spinning up %s:%s", opts.Repository, opts.Tag)
	var resource *dockertest.Resource
	resource, err := pool.RunWithOptions(&opts)
	if err != nil {
		return nil, fmt.Errorf("could not start %s %s", opts.Repository, err)
	}
	return resource, nil

}

func setupResource(pool *dockertest.Pool, repository string, tag string, name string, env []string) (*dockertest.Resource, error) {
	return setupResourceWithRunOptions(pool, dockertest.RunOptions{Name: name, Repository: repository, Tag: tag, Env: env})
}

func teardownResource(pool *dockertest.Pool, resource *dockertest.Resource) error {
	log.Info().Msgf("tearing down %s", resource.Container.Name)
	if err := pool.Purge(resource); err != nil {
		return fmt.Errorf("could not tear-down resource: %s", err)
	}
	return nil
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
		return nil, fmt.Errorf("could not connect to db: %s", err)
	}
	return db, nil
}
