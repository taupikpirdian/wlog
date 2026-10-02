package main

import (
	"context"
	"fmt"
	"os"
	"time"

	applicationactivity "github.com/taupikpirdian/wlog/application/activity"
	"github.com/taupikpirdian/wlog/application/bootstrap"
	applicationdashboard "github.com/taupikpirdian/wlog/application/dashboard"
	applicationhook "github.com/taupikpirdian/wlog/application/hook"
	applicationsession "github.com/taupikpirdian/wlog/application/session"
	"github.com/taupikpirdian/wlog/delivery/cli"
	"github.com/taupikpirdian/wlog/domain/ticket"
	"github.com/taupikpirdian/wlog/infrastructure/configfile"
	"github.com/taupikpirdian/wlog/infrastructure/gitcapture"
	"github.com/taupikpirdian/wlog/infrastructure/gitcontext"
	"github.com/taupikpirdian/wlog/infrastructure/githook"
	"github.com/taupikpirdian/wlog/infrastructure/storage"
)

var version = "dev"

func main() {
	root := cli.NewRootCommand(dashboardFactory(configfile.NewLoader(), time.Now, time.Local), version, openSessions, openNotes, openGitCaptures)
	root.AddCommand(cli.NewInstallHooksCommand(openHooks))
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func dashboardFactory(loader bootstrap.ConfigLoader, now func() time.Time, location *time.Location) cli.DashboardFactory {
	return func(ctx context.Context) (cli.DashboardReader, func() error, error) {
		config, err := loader.LoadOrCreate(ctx)
		if err != nil {
			return nil, nil, err
		}
		db, err := storage.OpenDatabase(ctx, config.DatabasePath)
		if err != nil {
			return nil, nil, err
		}
		return applicationdashboard.NewService(storage.NewSQLiteDashboardStore(db), now, location), db.Close, nil
	}
}

func openHooks(context.Context) (cli.HookInstaller, error) {
	directory, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return applicationhook.NewService(githook.NewInspector(directory), githook.NewStore()), nil
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
