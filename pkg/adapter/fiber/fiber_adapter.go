package fiber

import (
	"context"
	"errors"
	f "github.com/gofiber/fiber"
	"github.com/gofiber/fiber/middleware"
	"github.com/nferreira/adapter/pkg/adapter"
	"github.com/nferreira/app/pkg/app"
	"github.com/nferreira/app/pkg/env"
	"github.com/nferreira/app/pkg/service"
	"reflect"
	"strings"
	"time"
)

const (
	Payload       = service.Payload
	CorrelationId = "Correlation-Id"
	AdapterId     = "fiber"
)

var (
	ErrBadPayload = errors.New("Bad payload")
)

type Params map[string]interface{}
type Handler func(path string, handlers ...f.Handler) *f.Route
type GetParams func(fiberRule *BindingRule,
	businessService service.BusinessService,
	c *f.Ctx) (Params, error)

type Adapter struct {
	app      app.App
	fiberApp *f.App
}

func NewFiberAdapter() adapter.Adapter {
	return &Adapter{
		app:      nil,
		fiberApp: newFiber(),
	}
}

func (a *Adapter) BindRules(rules map[adapter.BindingRule]service.BusinessService) {
	for rule, businessService := range rules {
		fiberRule := rule.(*BindingRule)
		if fiberRule.Method == Get {
			bind(fiberRule, a, businessService, a.fiberApp.Get, getParams)
		} else if fiberRule.Method == Post {
			bind(fiberRule, a, businessService, a.fiberApp.Post, getPayload)
		} else if fiberRule.Method == Put {
			bind(fiberRule, a, businessService, a.fiberApp.Put, getPayload)
		} else if fiberRule.Method == Patch {
			bind(fiberRule, a, businessService, a.fiberApp.Patch, getPayload)
		} else if fiberRule.Method == Delete {
			bind(fiberRule, a, businessService, a.fiberApp.Delete, getParams)
		} else if fiberRule.Method == Options {
			bind(fiberRule, a, businessService, a.fiberApp.Options, getParams)
		}
	}
}

func (a *Adapter) Start(ctx context.Context) error {
	application := ctx.Value("app")
	switch application.(type) {
	case app.App:
		a.app = application.(app.App)
	default:
		panic("I need an application to live!!!")
	}
	return a.fiberApp.Listen(env.GetInt("FIBER_HTTP_PORT", 8080))
}

func (a *Adapter) Stop(_ context.Context) error {
	return a.fiberApp.Shutdown()
}

func (a *Adapter) CheckHealth(ctx context.Context) error {
	return nil
}

func newFiber() *f.App {
	fiberApp := f.New(&f.Settings{
		Concurrency:     env.GetInt("FIBER_CONCURRENCY", 256*1024),
		ReadTimeout:     env.GetDuration("FIBER_READ_TIMEOUT", time.Duration(3)*time.Minute),
		WriteTimeout:    env.GetDuration("FIBER_WRITER_TIMEOUT", time.Duration(3)*time.Minute),
		ReadBufferSize:  env.GetInt("FIBER_READ_BUFFER", 4096),
		WriteBufferSize: env.GetInt("FIBER_WRITE_BUFFER", 4096),
	})

	fiberApp.Use(middleware.Logger("${time} ${method} ${path} - ${ip} - ${status} - ${latency}\n"))

	if env.GetBool("FIBER_USE_COMPRESSION", false) {
		fiberApp.Use(middleware.Compress(middleware.CompressLevelBestSpeed))
	}

	return fiberApp
}

func getParams(fiberRule *BindingRule, businessService service.BusinessService, c *f.Ctx) (Params, error) {
	params := make(Params)
	for _, param := range fiberRule.Params {
		var value string
		value = c.Params(param)
		if strings.TrimSpace(value) == "" {
			value = strings.TrimSpace(c.Query(param))
		}
		params[param] = value
	}
	return params, nil
}

func getPayload(fiberRule *BindingRule, businessService service.BusinessService, c *f.Ctx) (params Params, err error) {
	params, err = getParams(fiberRule, businessService, c)
	if err != nil {
		return nil, err
	}
	serviceRequest := businessService.CreateRequest()
	err = c.BodyParser(&serviceRequest)
	if err == nil {
		params[Payload] = serviceRequest
		return params, nil
	}
	return nil, ErrBadPayload
}

func bind(fiberRule *BindingRule,
	a *Adapter,
	businessService service.BusinessService,
	handler Handler,
	getParams GetParams) {

	handler(fiberRule.Path, func(c *f.Ctx) {
		params, err := getParams(fiberRule, businessService, c)
		if err != nil {
			a.handleResult(c,
				service.
					NewResultBuilder().
					WithError(err).
					Build(),
				fiberRule)
			return
		}

		result, done := a.executeBusinessService(c, businessService, params, fiberRule)
		if done {
			return
		}
		a.handleResult(c, result, fiberRule)
	})
}

func (a *Adapter) executeBusinessService(c *f.Ctx, businessService service.BusinessService, params Params, fiberRule *BindingRule) (*service.Result, bool) {
	correlationId := GetCorrelationId(c)
	executionContext := service.NewExecutionContext(correlationId, a.app)
	ctx := context.WithValue(c.Context(), service.ExecutionContextKey, executionContext)
	result := businessService.Execute(ctx, service.Params(map[string]interface{}(params)))
	if result.Error != nil {
		k := reflect.TypeOf(result.Error).Kind()
		hashable := k < reflect.Array || k == reflect.Ptr || k == reflect.UnsafePointer
		if hashable {
			status, found := fiberRule.ErrorMapping[result.Error]
			if found {
				c.Status(status).Send()
			} else {
				if result.Code == 0 {
					c.Status(500).Send()
				} else {
					c.Status(result.Code).Send()
				}
			}
		} else {
			if result.Code == 0 {
				c.Status(500).Send()
			} else {
				c.Status(result.Code).Send()
			}
		}

		return result, true
	}
	return result, false
}

func (a *Adapter) handleResult(c *f.Ctx, result *service.Result, fiberRule *BindingRule) {
	if result.Error != nil {
		status, found := fiberRule.ErrorMapping[result.Error]
		if found {
			c.Status(status).Send()
		} else {
			// TODO, Add a Service Code mapping to Status Code
			// This need to be done in the rule binding object
			if result.Code == 0 {
				c.Status(500).Send()
			} else {
				c.Status(result.Code).Send()
			}
		}
	} else {
		if (result.Code == 0) {
			c.Status(500).Send()
		} else {
			c.Status(result.Code)
		}
		for key, value := range result.Headers {
			v, err := ToString(value)
			if err != nil {
				c.Status(500).Send(err.Error())
				return
			}
			c.Set(key, *v)
		}
		if (result.Response != nil) {
			err := c.JSON(result.Response)
			if err != nil {
				c.Status(500).Send()
			}
		}
	}
}
