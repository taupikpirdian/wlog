package main

import (
	"context"
	"fmt"
	"os"
	"time"

	applicationactivity "github.com/taupikpirdian/wlog/application/activity"
	"github.com/taupikpirdian/wlog/application/bootstrap"
	applicationsession "github.com/taupikpirdian/wlog/application/session"
	"github.com/taupikpirdian/wlog/delivery/cli"
	"github.com/taupikpirdian/wlog/domain/ticket"
	"github.com/taupikpirdian/wlog/infrastructure/configfile"
	"github.com/taupikpirdian/wlog/infrastructure/gitcapture"
	"github.com/taupikpirdian/wlog/infrastructure/gitcontext"
	"github.com/taupikpirdian/wlog/infrastructure/storage"
)

var version = "dev"

func main() {
	initializer := bootstrap.NewService(
		configfile.NewLoader(),
		storage.NewSQLiteInitializer(),
	)
	root := cli.NewRootCommand(initializer, version, openSessions, openNotes, openGitCaptures)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func openGitCaptures(ctx context.Context) (cli.GitCaptureService, func() error, error) {
	config, err := configfile.NewLoader().LoadOrCreate(ctx)
	if err != nil {
		return nil, nil, err
	}
	pattern, err := ticket.NewKeyPattern(config.Ticket.Pattern)
	if err != nil {
		return nil, nil, err
	}
	directory, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	db, err := storage.OpenDatabase(ctx, config.DatabasePath)
	if err != nil {
		return nil, nil, err
	}
	options := applicationactivity.CaptureOptions{ChangedFiles: config.Git.CaptureChangedFiles, DiffStat: config.Git.CaptureDiffStat, FullDiff: config.Git.CaptureFullDiff}
	return applicationactivity.NewCaptureService(gitcapture.NewReader(directory), storage.NewSQLiteActivityStore(db), pattern, options, time.Now), db.Close, nil
}

func openNotes(ctx context.Context) (cli.NoteService, func() error, error) {
	config, err := configfile.NewLoader().LoadOrCreate(ctx)
	if err != nil {
		return nil, nil, err
	}
	db, err := storage.OpenDatabase(ctx, config.DatabasePath)
	if err != nil {
		return nil, nil, err
	}
	return applicationactivity.NewService(storage.NewSQLiteActivityStore(db), time.Now), db.Close, nil
}

func openSessions(ctx context.Context) (cli.SessionService, func() error, error) {
	config, err := configfile.NewLoader().LoadOrCreate(ctx)
	if err != nil {
		return nil, nil, err
	}
	pattern, err := ticket.NewKeyPattern(config.Ticket.Pattern)
	if err != nil {
		return nil, nil, err
	}
	db, err := storage.OpenDatabase(ctx, config.DatabasePath)
	if err != nil {
		return nil, nil, err
	}
	service := applicationsession.NewService(storage.NewSQLiteSessionStore(db), pattern, time.Now, gitcontext.Current)
	return service, db.Close, nil
}
