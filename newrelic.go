package nronthego

import (
	"net/http"
	"net/url"

	"github.com/gofiber/fiber/v3"
	"github.com/newrelic/go-agent/v3/newrelic"
)

var App *newrelic.Application

// InitNewRelic initializes NR once.  Turn on debug logging so we can
// see “Starting web transaction …” in stdout.
func InitNewRelic(appName, licenseKey string) error {
	var err error
	App, err = newrelic.NewApplication(
		newrelic.ConfigAppName(appName),
		newrelic.ConfigLicense(licenseKey),
		newrelic.ConfigDistributedTracerEnabled(true),
		// newrelic.ConfigDebugLogger(os.Stdout), // 🚩 debug logs
	)
	return err
}

// NewRelicMiddleware instruments each request as a Web txn using the raw path.
func NewRelicMiddleware(c fiber.Ctx) error {
	if App == nil {
		return c.Next()
	}

	// 1) Build a clean name from the raw request path:
	rawPath := string(c.Request().URI().Path())
	txnName := c.Method() + " " + rawPath
	// log.Printf("[NR] Starting txn %q", txnName)

	// 2) Start the txn
	txn := App.StartTransaction(txnName)

	// 3) Mark it as a web request
	req := c.Request()
	u := &url.URL{
		Path:     rawPath,
		RawQuery: string(req.URI().QueryString()),
	}
	hdr := make(http.Header)
	req.Header.VisitAll(func(k, v []byte) {
		hdr.Add(string(k), string(v))
	})
	txn.SetWebRequest(newrelic.WebRequest{
		Header:    hdr,
		URL:       u,
		Method:    string(req.Header.Method()),
		Transport: newrelic.TransportHTTP,
		Host:      string(req.URI().Host()),
	})

	// 4) Store txn in context for nested segments
	c.SetContext(newrelic.NewContext(c.Context(), txn))

	// 5) Call next handler
	err := c.Next()

	// 6) Record the status code as an attribute
	status := c.Response().StatusCode()
	txn.AddAttribute("http.statusCode", status)

	// 7) End the txn
	defer txn.End()
	//log.Printf("[NR] Ended txn %q (status=%d)", txnName, status)

	return err
}
