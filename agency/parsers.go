package agency

import (
	"encoding/json"
	"github.com/d0rc/agent-os/tools"
	"gopkg.in/yaml.v2"
	"reflect"
)

type AgentSettings struct {
	Agent *GeneralAgentSettings `yaml:"agent"`
}

type LifeCycleType string
type GeneralAgentSettings struct {
	Name            string                    `yaml:"name"`
	InputSink       interface{}               `yaml:"input-sink"`
	PromptBased     *PromptBasedAgentSettings `yaml:"prompt-based"`
	LifeCycleType   LifeCycleType             `yaml:"life-cycle-type"`
	LifeCycleLength int                       `yaml:"life-cycle-length"`
}

type ResponseFormatType map[string]interface{}
type PromptBasedAgentSettings struct {
	Prompt          string             `yaml:"prompt"`
	ResponseFormat  ResponseFormatType `yaml:"response-format"`
	ResponseParsers []ResponseParser   `yaml:"response-parsers"`
}

type ResponseParser struct {
	ParserPath interface{} `yaml:"path"`
	ResultTags []string    `yaml:"tags"`
}

type ResponseParserResult struct {
	Tags   []string
	Value  interface{}
	Path   interface{}
	Parser *ResponseParser
}

func (rpr *ResponseParserResult) HasAnyTags(v ...string) bool {
	for _, tag := range v {
		for _, t := range rpr.Tags {
			if t == tag {
				return true
			}
		}
	}

	return false
}

func ParseAgency(data []byte) ([]*AgentSettings, error) {
	var settings []*AgentSettings

	err := yaml.Unmarshal(data, &settings)
	if err != nil {
		return nil, err
	}

	return settings, nil
}

func (settings *AgentSettings) GetResponseJSONFormat() (string, error) {
	fixMap(settings.Agent.PromptBased.ResponseFormat)
	fixMap(settings.Agent.PromptBased.ResponseFormat)
	res, err := json.MarshalIndent(settings.Agent.PromptBased.ResponseFormat, "", "\t")

	return string(res), err
}

func fixMap(data map[string]interface{}) {
	for k, v := range data {
		switch v := v.(type) {
		case map[interface{}]interface{}:
			// Convert map[interface{}]interface{} to map[string]interface{}
			convertedData := make(map[string]interface{})
			for k, v := range v {
				convertedData[k.(string)] = v
			}
			data[k] = convertedData
		case map[string]interface{}:
			// If the value is a map, recursively fix it
			fixMap(v)
		case reflect.Type:
			// If the value is a reflect.Type, handle it appropriately
			data[k] = v.String()
		}
	}
}

func (settings *AgentSettings) ParseResponse(response string) ([]*ResponseParserResult, error) {
	// step one is parse JSON itself, according to the schema
	var parsedResponse ResponseFormatType
	err := tools.ParseJSON(response, func(response string) error {
		return json.Unmarshal([]byte(response), &parsedResponse)
	})
	if err != nil {
		return nil, err
	}

	results := make([]*ResponseParserResult, 0)

	// pick data according to configured parsers
	for _, parser := range settings.Agent.PromptBased.ResponseParsers {
		if _, ok := parser.ParserPath.(string); ok {
			// it's a string, so it should be simple, just pick it
			if obj := parsedResponse[parser.ParserPath.(string)]; obj != nil {
				results = append(results, &ResponseParserResult{
					Tags:   parser.ResultTags,
					Value:  obj,
					Path:   parser.ParserPath,
					Parser: &parser,
				})
			}
		}

		if pathList, ok := parser.ParserPath.([]string); ok {
			// it's a list of strings, which are map[string]interface{} keys
			// let's dive into the map
			var obj interface{}
			obj = parsedResponse
			for _, path := range pathList {
				if obj == nil {
					break
				}

				if objMap, ok := obj.(map[string]interface{}); ok {
					obj = objMap[path]
				} else {
					obj = nil
				}
			}

			if obj != nil {
				results = append(results, &ResponseParserResult{
					Tags:   parser.ResultTags,
					Value:  obj,
					Path:   parser.ParserPath,
					Parser: &parser,
				})
			}
		}
	}

	return results, nil
}