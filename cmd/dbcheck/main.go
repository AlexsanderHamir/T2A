package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks"
)

const dbTimeout = 30 * time.Second

type options struct {
	migrate bool
	envPath string
}

func parseFlags() options {
	var o options
	flag.BoolVar(&o.migrate, "migrate", false, "run GORM AutoMigrate after connecting")
	flag.StringVar(&o.envPath, "env", "", "path to .env (default: <repo-root>/.env)")
	flag.Parse()
	return o
}

func run(o options) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	envFile, err := resolveDotenvPath(wd, o.envPath)
	if err != nil {
		return err
	}
	if err := loadDotenv(envFile); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	db, err := connectAndPing(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	log.Print("connected")

	if o.migrate {
		if err := tasks.MigratePostgreSQL(ctx, db); err != nil {
			return err
		}
		log.Print("migrate OK")
	}
	return nil
}

func main() {
	if err := run(parseFlags()); err != nil {
		log.Fatal(err)
	}
}
