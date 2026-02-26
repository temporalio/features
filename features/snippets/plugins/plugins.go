package plugins

import (
	"context"

	"github.com/nexus-rpc/sdk-go/nexus"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// @@@SNIPSTART go-plugin-activity
func SomeActivity(ctx context.Context) error {
	// Activity implementation
	return nil
}

func createActivityPlugin() (*temporal.SimplePlugin, error) {
	return temporal.NewSimplePlugin(temporal.SimplePluginOptions{
		Name: "PluginName",
		RunContextBefore: func(ctx context.Context, options temporal.SimplePluginRunContextBeforeOptions) error {
			options.Registry.RegisterActivityWithOptions(
				SomeActivity,
				activity.RegisterOptions{Name: "SomeActivity"},
			)
			return nil
		},
	})
}

// @@@SNIPEND

// @@@SNIPSTART go-plugin-workflow
func HelloWorkflow(ctx workflow.Context, name string) (string, error) {
	return "Hello, " + name + "!", nil
}

func createWorkflowPlugin() (*temporal.SimplePlugin, error) {
	return temporal.NewSimplePlugin(temporal.SimplePluginOptions{
		Name: "PluginName",
		RunContextBefore: func(ctx context.Context, options temporal.SimplePluginRunContextBeforeOptions) error {
			options.Registry.RegisterWorkflowWithOptions(
				HelloWorkflow,
				workflow.RegisterOptions{Name: "HelloWorkflow"},
			)
			return nil
		},
	})
}

// @@@SNIPEND

// @@@SNIPSTART go-plugin-nexus
type WeatherInput struct {
	City string `json:"city"`
}

type Weather struct {
	City             string `json:"city"`
	TemperatureRange string `json:"temperatureRange"`
	Conditions       string `json:"conditions"`
}

var WeatherService = nexus.NewService("weather-service")

var GetWeatherOperation = nexus.NewSyncOperation(
	"get-weather",
	func(ctx context.Context, input WeatherInput, options nexus.StartOperationOptions) (Weather, error) {
		return Weather{
			City:             input.City,
			TemperatureRange: "14-20C",
			Conditions:       "Sunny with wind.",
		}, nil
	},
)

func createNexusPlugin() (*temporal.SimplePlugin, error) {
	return temporal.NewSimplePlugin(temporal.SimplePluginOptions{
		Name: "PluginName",
		RunContextBefore: func(ctx context.Context, options temporal.SimplePluginRunContextBeforeOptions) error {
			options.Registry.RegisterNexusService(WeatherService)
			return nil
		},
	})
}

// @@@SNIPEND

// @@@SNIPSTART go-plugin-converter
func createConverterPlugin() (*temporal.SimplePlugin, error) {
	customConverter := converter.GetDefaultDataConverter() // Or your custom converter
	
	return temporal.NewSimplePlugin(temporal.SimplePluginOptions{
		Name:          "PluginName",
		DataConverter: customConverter,
	})
}

// @@@SNIPEND

// @@@SNIPSTART go-plugin-interceptors
type SomeWorkerInterceptor struct {
	interceptor.WorkerInterceptorBase
}

type SomeClientInterceptor struct {
	interceptor.ClientInterceptorBase
}

func createInterceptorPlugin() (*temporal.SimplePlugin, error) {
	return temporal.NewSimplePlugin(temporal.SimplePluginOptions{
		Name:               "PluginName",
		WorkerInterceptors: []interceptor.WorkerInterceptor{&SomeWorkerInterceptor{}},
		ClientInterceptors: []interceptor.ClientInterceptor{&SomeClientInterceptor{}},
	})
}

// @@@SNIPEND