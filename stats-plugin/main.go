package main

import (
	"fmt"

	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/tidwall/gjson"
)

// pluginContext implements types.PluginContext interface of proxy-wasm-go SDK.
type PluginContext struct {
	// Embed the default plugin context here,
	// so that we don't need to reimplement all the methods.
	types.DefaultPluginContext
}

func (ctx *PluginContext) OnPluginStart(pluginConfigurationSize int) types.OnPluginStartStatus {
	return types.OnPluginStartStatusOK
}

type StatsPluginHttpContext struct {
	types.DefaultHttpContext
}

// Override types.DefaultPluginContext.
func (ctx *PluginContext) NewHttpContext(contextID uint32) types.HttpContext {
	return &StatsPluginHttpContext{}
}

func main() {
	// SetVMContext is the entrypoint for setting up this entire Wasm VM.
	// Please make sure that this entrypoint be called during "main()" function,
	// otherwise this VM would fail.
	proxywasm.SetVMContext(&vmContext{})
}

// vmContext implements types.VMContext interface of proxy-wasm-go SDK.
type vmContext struct {
	// Embed the default VM context here,
	// so that we don't need to reimplement all the methods.
	types.DefaultVMContext
}

// Override types.DefaultVMContext.
func (*vmContext) NewPluginContext(contextID uint32) types.PluginContext {
	return &PluginContext{}
}

func (ctx *StatsPluginHttpContext) OnHttpResponseBody(bodySize int, endOfStream bool) types.Action {
	// Wait for the entire response body.
	if !endOfStream {
		return types.ActionPause
	}

	body, err := proxywasm.GetHttpResponseBody(0, bodySize)
	if err != nil {
		proxywasm.LogErrorf("failed to get response body: %v", err)
		return types.ActionContinue
	}

	// Parse the body as json and validate it.
	jsonBody := gjson.ParseBytes(body)

	// Sample response:
	// {
	// 	"predictions": [
	// 	  [
	// 		{ "label": "joy", "score": 0.9889927506446838 },
	// 		{ "label": "love", "score": 0.004175746813416481 },
	// 		{ "label": "anger", "score": 0.003846140578389168 },
	// 		{ "label": "sadness", "score": 0.0012988817179575562 },
	// 		{ "label": "fear", "score": 0.0009755274513736367 },
	// 		{ "label": "surprise", "score": 0.0007109164143912494 }
	// 	  ]
	// 	]
	//   }

	// Get the predictions
	rootArray := jsonBody.Get("predictions")
	if !rootArray.Exists() {
		proxywasm.LogError("missing predictions key from response")
		return types.ActionContinue
	}

	// Get the first element from the root array
	predictions := rootArray.Array()[0]
	if !predictions.Exists() {
		proxywasm.LogError("missing predictions key from response")
		return types.ActionContinue
	}

	highestScore := float64(0)
	highestLabel := ""
	for _, prediction := range predictions.Array() {
		// prediction looks like this: { "label": "joy", "score": 0.9889927506446838 }
		label := prediction.Get("label").String()
		score := prediction.Get("score").Float()
		if score > highestScore {
			highestScore = score
			highestLabel = label
		}
	}

	proxywasm.LogInfof("highest label: %v, highest score: %v", highestLabel, highestScore)

	// Create a metric using the label as a name and increment the counter
	metric := GetOrCreateMetric(highestLabel, nil)
	metric.Increment(1)
	return types.ActionContinue
}

// Collection of counter metrics
var counters = map[string]proxywasm.MetricCounter{}

// getOrCreateMetric is a helper function to get or create a metric with a specific name
// name is the "short" metric name, e.g. "total_tokens"
// tags are optional. note that the tags must be provided in the stats_tags field in the envoy config
func GetOrCreateMetric(name string, tags map[string]string) proxywasm.MetricCounter {
	// Create the metric name by combining the name and the provided tags
	metricName := name
	for k, v := range tags {
		metricName += fmt.Sprintf("_%s=%s", k, v)
	}

	metric, ok := counters[metricName]
	if !ok {
		metric = proxywasm.DefineCounterMetric(metricName)
		counters[metricName] = metric
	}
	return metric
}
