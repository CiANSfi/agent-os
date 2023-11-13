package engines

import (
	"strings"
	"time"
)

/*

Description is generated by bloop.ai from vllm source code:

JSON exmaple:

{
  "prompts": ["San Francisco is a", "New York is a"],
  "n": 1,
  "stream": false,
  "log_level": "info",
  "logprobs": 10,
  "echo": false,
  "max_tokens": 16,
  "temperature": 0.0,
  "top_p": 1.0,
  "presence_penalty": 0.0,
  "frequency_penalty": 0.0,
  "best_of": 1,
  "stop_sequences": ["\n"]
}

In this example:

"prompts" is a list of texts you want the model to continue.
"n" is the number of completions to generate for each prompt.
"stream" is a boolean indicating whether to stream the results or not.
"log_level" is the logging level.
"logprobs" is the number of most probable tokens to log for each token.
"echo" is a boolean indicating whether to include the prompt in the response.
"max_tokens" is the maximum number of tokens in the generated text.
"temperature" controls the randomness of the model's output. A higher value (closer to 1) makes the output more random, while a lower value (closer to 0) makes it more deterministic.
"top_p" is used for nucleus sampling and controls the cumulative probability cutoff.
"presence_penalty" and "frequency_penalty" are advanced parameters that control the model's output.
"best_of" is the number of times to run the model and keep the best result.
"stop_sequences" is a list of sequences where the API will stop generating further tokens.
Please note that the actual parameters may vary depending on the specific implementation of the vLLM engine.

*/

const InferenceTimeout = 600 * time.Second

type RemoteInferenceEngine struct {
	EndpointUrl           string
	EmbeddingsEndpointUrl string
	MaxBatchSize          int
	Performance           float32 // tokens per second
	MaxRequests           int
	Models                []string // supported models
	RequestsServed        uint64
	TimeConsumed          time.Duration
	TokensProcessed       uint64
	TokensGenerated       uint64
	PromptTokens          uint64
	LeasedAt              time.Time
	Busy                  bool
	EmbeddingsDims        *uint64
	CompletionFailed      bool
	EmbeddingsFailed      bool
}

func StartInferenceEngine(engine *RemoteInferenceEngine, done chan struct{}) {
	// we need to send a completion request to the engine
	// detect the model, then send embeddings request to the engine and
	// detect the model and dimensions
	_, err := RunCompletionRequest(engine, []*JobQueueTask{
		{
			Req: &GenerationSettings{RawPrompt: "### Instruction\nProvide an answer. 2 + 2 = ?\n### Assistant: ", MaxRetries: 1, Temperature: 0.1},
		},
	})
	if err != nil {
		// engine failed to run completion
		engine.CompletionFailed = true
	}

	cEmb, err := RunEmbeddingsRequest(engine, []*JobQueueTask{
		{
			Req: &GenerationSettings{RawPrompt: "Hello world", MaxRetries: 1, Temperature: 0.1},
		},
	})
	if err != nil {
		// engine failed to run embeddings
		engine.EmbeddingsFailed = true
	}

	if len(cEmb) > 0 {
		if cEmb[0].Model != nil {
			parsedModelName := parseModelName(*cEmb[0].Model)
			added := false
			for idx, model := range engine.Models {
				if model == parsedModelName || model == "" {
					added = true
					engine.Models[idx] = parsedModelName
					break
				}
			}
			if !added {
				engine.Models = append(engine.Models, parsedModelName)
			}
		}
		if len(cEmb[0].VecF64) > 0 {
			dims := uint64(len(cEmb[0].VecF64))
			engine.EmbeddingsDims = &dims
		}
	}

	done <- struct{}{}
}

func parseModelName(s string) string {
	// /Users/ds/.cache/lm-studio/models/TheBloke/dolphin-2.2.1-mistral-7B-GGUF/dolphin-2.2.1-mistral-7b.Q6_K.gguf
	if strings.HasSuffix(s, ".gguf") {
		// it's a filename, if there's a path, take the last two directories
		// in this case, it's going to be "TheBloke/dolphin-2.2.1-mistral-7B-GGUF"
		// if there's no path, take the last directory
		// in this case, it's going to be "dolphin-2.2.1-mistral-7B-GGUF"
		var modelName string

		if strings.Contains(s, "/") {
			parts := strings.Split(s, "/")
			// last element is a filename, we don't need it
			// just names of two directories, or even one, if it's in the root
			// or something
			if len(parts) > 2 {
				// now starting with parts[len(parts)-3:] we have the last three
				// including file name, which we do not need
				modelName = strings.Join(parts[len(parts)-3:], "/")
				// drop filename now
				modelName = modelName[:len(modelName)-len(parts[len(parts)-1])]
			} else {
				modelName = parts[len(parts)-1]
			}
		} else {
			modelName = s
		}

		if strings.HasSuffix(modelName, ".gguf") {
			modelName = modelName[:len(modelName)-5]
		}

		// drop the trailing slash
		if strings.HasSuffix(modelName, "/") {
			modelName = modelName[:len(modelName)-1]
		}

		return modelName
	}

	return s
}
