package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"time"

	realcf "github.com/cloudfoundry-community/go-cfclient"
	"github.com/gammazero/workerpool"
	migrate "github.com/rubenv/sql-migrate"

	_ "github.com/lib/pq"

	"github.com/starkandwayne/scheduler-for-ocf/cf"
	"github.com/starkandwayne/scheduler-for-ocf/combined"
	"github.com/starkandwayne/scheduler-for-ocf/core"
	"github.com/starkandwayne/scheduler-for-ocf/cron"
	"github.com/starkandwayne/scheduler-for-ocf/http"
	"github.com/starkandwayne/scheduler-for-ocf/logger"
	"github.com/starkandwayne/scheduler-for-ocf/postgres"
	"github.com/starkandwayne/scheduler-for-ocf/postgres/migrations"
)

var callRunner = http.NewRunService()

func meat() int {
	log := logger.New()
	tag := "scheduler-for-ocf"

	port := 8000
	portEnv := os.Getenv("SCHEDULER_PORT")
	if len(portEnv) > 0 {
		if t, err := strconv.Atoi(portEnv); err == nil {
			port = t
		}
	}

	clientID := os.Getenv("CLIENT_ID")
	if len(clientID) == 0 {
		log.Error(tag, "CLIENT_ID not set")
		return 255
	}

	clientSecret := os.Getenv("CLIENT_SECRET")
	if len(clientSecret) == 0 {
		log.Error(tag, "CLIENT_SECRET not set")
		return 255
	}

	dbURL := os.Getenv("DATABASE_URL")
	if len(dbURL) == 0 {
		log.Error(tag, "DATABASE_URL not set")
		return 255
	}

	cfEndpoint := os.Getenv("CF_ENDPOINT")
	if len(cfEndpoint) == 0 {
		log.Error(tag, "CF_ENDPOINT not set")
		return 255
	}

	uaaEndpoint := os.Getenv("UAA_ENDPOINT")
	if len(uaaEndpoint) == 0 {
		log.Error(tag, "UAA_ENDPOINT not set")
		return 255
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(fmt.Sprintf("could not open the database: %s", err.Error()))
	}
	defer db.Close()

	_, err = migrate.Exec(db, "postgres", migrations.Collection, migrate.Up)
	if err != nil {
		log.Error(tag, fmt.Sprintf("could not update database schema: %s", err.Error()))
		return 255
	}

	cfg := &realcf.Config{
		ClientID:          clientID,
		ClientSecret:      clientSecret,
		ApiAddress:        cfEndpoint,
		SkipSslValidation: true,
	}

	log.Info(tag, "trying to instantiate a cf client")

	cfclient, err := realcf.NewClient(cfg)
	if err != nil {
		log.Error(tag, fmt.Sprintf("could not instantiate cf client: %s", err.Error()))
		return 255
	}

	log.Info(tag, "got the cf client set up")

	auth := cf.NewAuthService(cfclient)
	jobs := postgres.NewJobService(db)
	calls := postgres.NewCallService(db)
	info := cf.NewInfoService(cfclient)
	jobRunner := cf.NewRunService(cfclient)
	schedules := postgres.NewScheduleService(db)
	executions := postgres.NewExecutionService(db)
	runner := combined.NewRunService(
		map[string]core.RunService{
			"job":  jobRunner,
			"call": callRunner,
		},
	)

	workers := workerpool.New(10)
	defer workers.StopWait()

	cronService := cron.NewCronService(log)
	cronService.Start()
	defer cronService.Stop()

	services := &core.Services{
		Jobs:       jobs,
		Calls:      calls,
		Info:       info,
		Schedules:  schedules,
		Workers:    workers,
		Runner:     runner,
		Executions: executions,
		Cron:       cronService,
		Logger:     log,
		Auth:       auth,
	}

	// Load up all existing schedules
	log.Info(tag, "loading existing schedules")
	for _, schedule := range schedules.Enabled() {
		if schedule.RefType == "job" {
			if job, err := jobs.Get(schedule.RefGUID); err == nil {
				log.Info(
					tag,
					fmt.Sprintf(
						"loading job schedule for %s (%s)",
						job.Name,
						schedule.Expression,
					),
				)

				cronService.Add(core.NewJobRun(job, schedule, services))
			}
		} else {
			if call, err := calls.Get(schedule.RefGUID); err == nil {
				log.Info(
					tag,
					fmt.Sprintf(
						"loading call schedule for %s (%s)",
						call.Name,
						schedule.Expression,
					),
				)

				cronService.Add(core.NewCallRun(call, schedule, services))
			}
		}
	}

	server := http.Server(fmt.Sprintf("0.0.0.0:%d", port), services)

	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Info(tag, "stopping the server")
		}
	}()

	log.Info(tag, fmt.Sprintf("listening for connections on %s", server.Addr))

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)

	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		server.Close()
		log.Error(tag, err.Error())
		return 2
	}

	return 0
}

func main() {
	os.Exit(meat())
}
