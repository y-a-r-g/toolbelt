package toolbelt

import (
	"github.com/sirupsen/logrus"
	"github.com/snowzach/rotatefilehook"
	"github.com/y-a-r-g/json2flag"
	"github.com/zbindenren/logrus_mail"
	"os"
	"reflect"
	"strconv"
	"strings"
)

//TODO: extract mail dependency to mail tool

type LogrusTool struct {
	*logrus.Logger
	config *LogrusConfig
}

type LogrusConfig struct {
	Level      string
	Filename   string
	MaxSize    uint64
	MaxBackups uint64
	MaxAge     uint
	Email      string

	SmtpAddress  string
	SmtpUsername string
	SmtpPassword string
	SmtpFrom     string
}

var TLogrus = reflect.TypeOf(LogrusTool{})

func (t *LogrusTool) Configure(config ...interface{}) {
	t.config = &LogrusConfig{
		Level:      "warn",
		Filename:   "./.logs/log.txt",
		MaxSize:    100,
		MaxBackups: 0,
		MaxAge:     31,
	}

	if len(config) > 0 {
		*t.config = *config[0].(*LogrusConfig)
	}

	json2flag.FlagPrefixed(t.config, map[string]string{
		"Level":        "logging level. one of panic|fatal|error|warn|info|debug|trace",
		"Filename":     "name of the file to write logs to",
		"MaxSize":      "maximum log file size before it backuped (mb)",
		"MaxBackups":   "maximum backups number",
		"MaxAge":       "maximum number of days to retain old log files",
		"Email":        "email that will receive logs",
		"SmtpAddress":  "smtp server address and port",
		"SmtpUsername": "smtp server username",
		"SmtpPassword": "smtp server password",
		"SmtpFrom":     "from this email logs will be sent",
	}, TLogrus.Name())
}

func (t *LogrusTool) Dependencies() []reflect.Type {
	return nil
}

func (t *LogrusTool) Start(belt IBelt) {
	t.Logger = logrus.New()

	t.Formatter = &logrus.TextFormatter{}
	t.Hooks = make(logrus.LevelHooks)
	t.ExitFunc = os.Exit
	t.ReportCaller = false

	switch strings.ToLower(t.config.Level) {
	case "panic":
		t.Level = logrus.PanicLevel
		break
	case "fatal":
		t.Level = logrus.FatalLevel
		break
	case "error":
		t.Level = logrus.ErrorLevel
		break
	case "warn":
		t.Level = logrus.WarnLevel
		break
	case "info":
		t.Level = logrus.InfoLevel
		break
	case "debug":
		t.Level = logrus.DebugLevel
		break
	case "trace":
		t.Level = logrus.TraceLevel
		break
	default:
		t.Level = logrus.WarnLevel
	}

	if t.config.Filename == "" {
		t.Out = os.Stdout
	} else {
		hook, err := rotatefilehook.NewRotateFileHook(rotatefilehook.RotateFileConfig{
			Filename:   t.config.Filename,
			MaxSize:    int(t.config.MaxSize),
			MaxBackups: int(t.config.MaxBackups),
			MaxAge:     int(t.config.MaxAge),
			Level:      t.Level,
			Formatter:  &logrus.TextFormatter{},
		})
		if err != nil {
			panic(err)
		}
		t.Hooks.Add(hook)
	}

	if t.config.Email != "" {
		smtp := strings.Split(t.config.SmtpAddress, ":")
		if len(smtp) != 2 {
			panic("invalid smtp address")
		}
		var port int
		port, err := strconv.Atoi(smtp[1])
		if err != nil {
			panic(err)
		}

		hook, err := logrus_mail.NewMailAuthHook("Server log", smtp[0], port, t.config.SmtpFrom, t.config.Email, t.config.SmtpUsername, t.config.SmtpPassword)
		if err != nil {
			panic(err)
		}
		t.Hooks.Add(hook)
	}
}

func (t *LogrusTool) Stop(belt IBelt) {
}
