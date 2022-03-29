package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/gammazero/workerpool"
	migrate "github.com/rubenv/sql-migrate"

	_ "github.com/lib/pq"

	"github.com/starkandwayne/scheduler-for-ocf/combined"
	"github.com/starkandwayne/scheduler-for-ocf/core"
	"github.com/starkandwayne/scheduler-for-ocf/cron"
	"github.com/starkandwayne/scheduler-for-ocf/http"
	"github.com/starkandwayne/scheduler-for-ocf/logger"
	"github.com/starkandwayne/scheduler-for-ocf/mock"
	"github.com/starkandwayne/scheduler-for-ocf/postgres"
	"github.com/starkandwayne/scheduler-for-ocf/postgres/migrations"
)

var callRunner = http.NewRunService()
var jobRunner = mock.NewRunService()

func main() {
	log := logger.New()
	tag := "scheduler-for-ocf"

	dbURL := os.Getenv("DATABASE_URL")
	if len(dbURL) == 0 {
		log.Error(tag, "DATABASE_URL not set")
		os.Exit(255)
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(fmt.Sprintf("could not open the database: %s", err.Error()))
	}
	defer db.Close()

	_, err = migrate.Exec(db, "postgres", migrations.Collection, migrate.Up)
	if err != nil {
		log.Error(tag, "could not update database schema")
		os.Exit(255)
	}

	auth := mock.NewAuthService()
	jobs := postgres.NewJobService(db)
	calls := postgres.NewCallService(db)
	environment := mock.NewEnvironmentInfoService()
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
		Jobs:        jobs,
		Calls:       calls,
		Environment: environment,
		Schedules:   schedules,
		Workers:     workers,
		Runner:      runner,
		Executions:  executions,
		Cron:        cronService,
		Logger:      log,
		Auth:        auth,
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

	server := http.Server("0.0.0.0:8000", services)

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
		os.Exit(2)
	}
}
