package pkg

import (
	"fmt"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

func InitLogger(appName, licenseKey string) {
	Log = logrus.New()
	Log.SetFormatter(&logrus.JSONFormatter{})
	Log.SetOutput(os.Stdout)

	if licenseKey != "" {
		hook := NewNewRelicHook(licenseKey, appName)
		Log.AddHook(hook)
	}
}

// buildFields centralizes your requestTag + New Relic trace/span metadata,
// and also injects a “name” field as METHOD + PATH
func buildFields(c fiber.Ctx) logrus.Fields {
	fields := logrus.Fields{
		"request": fmt.Sprintf("%s %s", c.Method(), c.Path()),
		"source":  c.BaseURL(),
	}
	if txn := newrelic.FromContext(c.Context()); txn != nil {
		md := txn.GetLinkingMetadata()
		fields["trace.id"] = md.TraceID
		fields["span.id"] = md.SpanID
	}
	return fields
}

func INFO(c fiber.Ctx, args ...interface{}) {
	Log.WithFields(buildFields(c)).Info(args...)
}

func WARN(c fiber.Ctx, args ...interface{}) {
	Log.WithFields(buildFields(c)).Warn(args...)
}

func ERROR(c fiber.Ctx, args ...interface{}) {
	Log.WithFields(buildFields(c)).Error(args...)
}
