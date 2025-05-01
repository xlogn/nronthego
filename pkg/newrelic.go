package pkg

import (
	"github.com/gofiber/fiber/v3"
	"github.com/newrelic/go-agent/v3/newrelic"
)

var App *newrelic.Application

func InitNewRelic(appName, licenseKey string) error {
	var err error
	App, err = newrelic.NewApplication(
		newrelic.ConfigAppName(appName),
		newrelic.ConfigLicense(licenseKey),
		newrelic.ConfigDistributedTracerEnabled(true),
	)
	return err
}

func NewRelicMiddleware(c fiber.Ctx) error {
	if App == nil {
		return c.Next()
	}

	txn := App.StartTransaction(c.Route().Path)
	defer txn.End()

	// Add txn to request context
	ctx := newrelic.NewContext(c.Context(), txn)
	c.SetContext(ctx)

	// Proceed with the request
	return c.Next()
}
