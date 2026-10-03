package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	applicationactivity "github.com/taupikpirdian/wlog/application/activity"
	"github.com/taupikpirdian/wlog/application/bootstrap"
	applicationdashboard "github.com/taupikpirdian/wlog/application/dashboard"
	applicationenvironment "github.com/taupikpirdian/wlog/application/environment"
	applicationhook "github.com/taupikpirdian/wlog/application/hook"
	applicationsession "github.com/taupikpirdian/wlog/application/session"
	applicationsummary "github.com/taupikpirdian/wlog/application/summary"
	"github.com/taupikpirdian/wlog/delivery/cli"
	"github.com/taupikpirdian/wlog/domain/ticket"
	"github.com/taupikpirdian/wlog/infrastructure/aiagent"
	"github.com/taupikpirdian/wlog/infrastructure/aiskills"
	"github.com/taupikpirdian/wlog/infrastructure/configfile"
	"github.com/taupikpirdian/wlog/infrastructure/envdetect"
	"github.com/taupikpirdian/wlog/infrastructure/gitcapture"
	"github.com/taupikpirdian/wlog/infrastructure/gitcontext"
	"github.com/taupikpirdian/wlog/infrastructure/gitevidence"
	"github.com/taupikpirdian/wlog/infrastructure/githook"
	"github.com/taupikpirdian/wlog/infrastructure/storage"
)

var version = "dev"

func main() {
	dashboard := dashboardFactory(configfile.NewLoader(), time.Now, time.Local)
	root := cli.NewRootCommand(dashboard, version, openSessions, openNotes, openGitCaptures)
	root.AddCommand(cli.NewStatusCommand(dashboard, openHookStatus))
	root.AddCommand(cli.NewInstallHooksCommand(openHooks))
	aiOptions := cli.SummaryAIOptions{Config: configfile.NewLoader(), Git: gitevidence.NewService(), Agents: aiagent.NewFactory(), Skills: aiskills.NewResolver()}
	root.AddCommand(cli.NewSummaryCommand(summaryFactory(configfile.NewLoader(), time.Now, time.Local), aiOptions))
	root.AddCommand(cli.NewConfigCommand(aiOptions.Config))
	root.AddCommand(cli.NewSkillsCommand(aiOptions.Config, aiOptions.Skills))
	root.AddCommand(cli.NewGenerateTicketCommand(summaryFactory(configfile.NewLoader(), time.Now, time.Local), aiOptions))
	cli.AddManualSessionCommands(root, manualSessionFactory(configfile.NewLoader(), time.Now, time.Local, gitcontext.Current))
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func summaryFactory(loader bootstrap.ConfigLoader, now func() time.Time, location *time.Location) cli.SummaryFactory {
	return func(ctx context.Context) (cli.SummaryReader, func() error, error) {
		config, err := loader.LoadOrCreate(ctx)
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
		git := gitevidence.NewService()
		detector := &applicationenvironment.Detector{Git: git, Extractors: envdetect.DefaultExtractors()}
		check := applicationsummary.NewEnvironmentCheck(git, detector)
		return applicationsummary.NewService(storage.NewSQLiteDashboardStore(db), gitcontext.NewEmailReader(directory), now, location, check), db.Close, nil
	}
}

func manualSessionFactory(loader bootstrap.ConfigLoader, now func() time.Time, location *time.Location, repository func(context.Context) string) cli.ManualSessionFactory {
	return func(ctx context.Context) (cli.ManualSessionService, func() error, error) {
		config, err := loader.LoadOrCreate(ctx)
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
		return applicationsession.NewManualService(storage.NewSQLiteSessionStore(db), pattern, now, location, repository), db.Close, nil
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
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return nil, err
	}
	return applicationhook.NewServiceWithExecutable(githook.NewInspector(directory), githook.NewStore(), executable), nil
}

func openHookStatus(context.Context) (cli.HookStatusReader, error) {
	directory, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return applicationhook.NewStatusService(githook.NewInspector(directory), githook.NewStore()), nil
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
