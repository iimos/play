package main

import (
	"context"
	"fmt"
	"github.com/jmorganca/ollama/api"
	"log"
	"os"
	"os/signal"
	"strings"
)

func generate(ctx context.Context, ollama *api.Client, opts Options, model, systemPrompt, userInput string) (string, error) {
	builder := strings.Builder{}
	err := ollama.Generate(ctx, &api.GenerateRequest{
		Model:     model,
		Prompt:    "User: [INST] " + userInput + " [/INST]\nAssistant: [PYTHON]",
		System:    systemPrompt,
		Template:  "",
		Context:   nil,
		Stream:    nil,
		Raw:       false,
		Format:    "",
		KeepAlive: nil,
		Images:    nil,
		Options:   opts.toMap(),
	}, func(r api.GenerateResponse) error {
		_, err := builder.WriteString(r.Response)
		return err
	})
	if err != nil {
		return "", err
	}
	return builder.String(), nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	go func() {
		<-ctx.Done()
		os.Exit(0)
	}()

	ollama, err := api.ClientFromEnvironment()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	opts := DefaultOptions()
	prompt := genPrompt()

	if len(os.Args) > 1 {
		input := strings.TrimSpace(strings.Join(os.Args[1:], " "))
		fmt.Printf("input: %s\n", input)

		prompt = strings.Replace(promptTemplate2, "{task}", input, 1)
		resp, err := generate(ctx, ollama, opts, "llama2", "", prompt)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		fmt.Println(prompt)
		fmt.Println(resp)
		fmt.Println("============================================================")

		prompt = resp + "Give me the program code. I want to see only code! No comments, no explanations, be strictly to the point^ I just want to copypaste your answer to python and run it! I cant spend my time tuning it."
		resp, err = generate(ctx, ollama, opts, "codellama", "You are highly experienced python programmer.", "Here is a task from your boss: "+prompt)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		//if i := strings.Index(resp, "```"); i > 0 {
		//	resp = resp[:i]
		//}
		fmt.Print(resp)
		fmt.Print("\n\n")
		return
	}

	//for {
	//	fmt.Print("> ")
	//	reader := bufio.NewReader(os.Stdin)
	//	input, err := reader.ReadString('\n')
	//	if err != nil {
	//		fmt.Println("Error reading input:", err)
	//		os.Exit(1)
	//	}
	//
	//	resp, err := generate(ctx, ollama, opts, prompt, input)
	//	if err != nil {
	//		log.Println(err)
	//		os.Exit(1)
	//	}
	//	fmt.Print(resp)
	//	fmt.Print("\n\n")
	//}
}

// Options descriptions can be seen on https://github.com/jmorganca/ollama/blob/main/docs/modelfile.md#valid-parameters-and-values.
//
// Borrowed from https://github.com/Mycoearthdome/AIMY/blob/main/AIMY.go
type Options struct {
	Runner

	// Predict options used at runtime
	NumKeep          int      `json:"num_keep,omitempty"`
	Seed             int      `json:"seed,omitempty"`
	NumPredict       int      `json:"num_predict,omitempty"`
	TopK             int      `json:"top_k,omitempty"`
	TopP             float32  `json:"top_p,omitempty"`
	TFSZ             float32  `json:"tfs_z,omitempty"`
	TypicalP         float32  `json:"typical_p,omitempty"`
	RepeatLastN      int      `json:"repeat_last_n,omitempty"`
	Temperature      float32  `json:"temperature,omitempty"`
	RepeatPenalty    float32  `json:"repeat_penalty,omitempty"`
	PresencePenalty  float32  `json:"presence_penalty,omitempty"`
	FrequencyPenalty float32  `json:"frequency_penalty,omitempty"`
	Mirostat         int      `json:"mirostat,omitempty"`
	MirostatTau      float32  `json:"mirostat_tau,omitempty"`
	MirostatEta      float32  `json:"mirostat_eta,omitempty"`
	PenalizeNewline  bool     `json:"penalize_newline,omitempty"`
	Stop             []string `json:"stop,omitempty"`
}

func (o Options) toMap() map[string]any {
	m := make(map[string]any, 30)
	if o.NumKeep != 0 {
		m["num_keep"] = o.NumKeep
	}
	if o.Seed != 0 {
		m["seed"] = o.Seed
	}
	if o.NumPredict != 0 {
		m["num_predict"] = o.NumPredict
	}
	if o.TopK != 0 {
		m["top_k"] = o.TopK
	}
	if o.TopP != 0 {
		m["top_p"] = o.TopP
	}
	if o.TFSZ != 0 {
		m["tfs_z"] = o.TFSZ
	}
	if o.TypicalP != 0 {
		m["typical_p"] = o.TypicalP
	}
	if o.RepeatLastN != 0 {
		m["repeat_last_n"] = o.RepeatLastN
	}
	if o.Temperature != 0 {
		m["temperature"] = o.Temperature
	}
	if o.RepeatPenalty != 0 {
		m["repeat_penalty"] = o.RepeatPenalty
	}
	if o.PresencePenalty != 0 {
		m["presence_penalty"] = o.PresencePenalty
	}
	if o.FrequencyPenalty != 0 {
		m["frequency_penalty"] = o.FrequencyPenalty
	}
	if o.Mirostat != 0 {
		m["mirostat"] = o.Mirostat
	}
	if o.MirostatTau != 0 {
		m["mirostat_tau"] = o.MirostatTau
	}
	if o.MirostatEta != 0 {
		m["mirostat_eta"] = o.MirostatEta
	}
	if o.PenalizeNewline {
		m["penalize_newline"] = o.PenalizeNewline
	}
	if len(o.Stop) > 0 {
		m["stop"] = o.Stop
	}

	// runner options
	if o.UseNUMA {
		m["numa"] = o.UseNUMA
	}
	if o.NumCtx != 0 {
		m["num_ctx"] = o.NumCtx
	}
	if o.NumBatch != 0 {
		m["num_batch"] = o.NumBatch
	}
	if o.NumGQA != 0 {
		m["num_gqa"] = o.NumGQA
	}
	if o.NumGPU != 0 {
		m["num_gpu"] = o.NumGPU
	}
	if o.MainGPU != 0 {
		m["main_gpu"] = o.MainGPU
	}
	if o.LowVRAM {
		m["low_vram"] = o.LowVRAM
	}
	if o.F16KV {
		m["f16_kv"] = o.F16KV
	}
	if o.LogitsAll {
		m["logits_all"] = o.LogitsAll
	}
	if o.VocabOnly {
		m["vocab_only"] = o.VocabOnly
	}
	if o.UseMMap {
		m["use_mmap"] = o.UseMMap
	}
	if o.UseMLock {
		m["use_mlock"] = o.UseMLock
	}
	if o.EmbeddingOnly {
		m["embedding_only"] = o.EmbeddingOnly
	}
	if o.RopeFrequencyBase != 0 {
		m["rope_frequency_base"] = o.RopeFrequencyBase
	}
	if o.RopeFrequencyScale != 0 {
		m["rope_frequency_scale"] = o.RopeFrequencyScale
	}
	if o.NumThread != 0 {
		m["num_thread"] = o.NumThread
	}
	return m
}

// Runner options which must be set when the model is loaded into memory
type Runner struct {
	UseNUMA            bool    `json:"numa,omitempty"`
	NumCtx             int     `json:"num_ctx,omitempty"`
	NumBatch           int     `json:"num_batch,omitempty"`
	NumGQA             int     `json:"num_gqa,omitempty"`
	NumGPU             int     `json:"num_gpu,omitempty"`
	MainGPU            int     `json:"main_gpu,omitempty"`
	LowVRAM            bool    `json:"low_vram,omitempty"`
	F16KV              bool    `json:"f16_kv,omitempty"`
	LogitsAll          bool    `json:"logits_all,omitempty"`
	VocabOnly          bool    `json:"vocab_only,omitempty"`
	UseMMap            bool    `json:"use_mmap,omitempty"`
	UseMLock           bool    `json:"use_mlock,omitempty"`
	EmbeddingOnly      bool    `json:"embedding_only,omitempty"`
	RopeFrequencyBase  float32 `json:"rope_frequency_base,omitempty"`
	RopeFrequencyScale float32 `json:"rope_frequency_scale,omitempty"`
	NumThread          int     `json:"num_thread,omitempty"`
}

func DefaultOptions() Options {
	return Options{
		// options set on request to runner
		NumPredict:       -1, //Maximum number of tokens to predict when generating text. (Default: 128, -1 = infinite generation, -2 = fill context)
		NumKeep:          0,
		Temperature:      1.0, //The temperature of the model. Increasing the temperature will make the model answer more creatively. (Default: 0.8)
		TopK:             40,  //Reduces the probability of generating nonsense. A higher value (e.g. 100) will give more diverse answers, while a lower value (e.g. 10) will be more conservative. (Default: 40)
		TopP:             1.0, //Works together with top-k. A higher value (e.g., 0.95) will lead to more diverse text, while a lower value (e.g., 0.5) will generate more focused and conservative text. (Default: 0.9)
		TFSZ:             1.0, //Tail free sampling is used to reduce the impact of less probable tokens from the output. A higher value (e.g., 2.0) will reduce the impact more, while a value of 1.0 disables this setting. (default: 1)
		TypicalP:         1.0,
		RepeatLastN:      64,  //Sets how far back for the model to look back to prevent repetition. (Default: 64, 0 = disabled, -1 = num_ctx)
		RepeatPenalty:    1.1, //Sets how strongly to penalize repetitions. A higher value (e.g., 1.5) will penalize repetitions more strongly, while a lower value (e.g., 0.9) will be more lenient. (Default: 1.1)
		PresencePenalty:  0.0,
		FrequencyPenalty: 0.0,
		Mirostat:         0,   //Enable Mirostat sampling for controlling perplexity. (default: 0, 0 = disabled, 1 = Mirostat, 2 = Mirostat 2.0)
		MirostatTau:      5.0, //Controls the balance between coherence and diversity of the output. A lower value will result in more focused and coherent text. (Default: 5.0)
		MirostatEta:      0.1, //Influences how quickly the algorithm responds to feedback from the generated text. A lower learning rate will result in slower adjustments, while a higher learning rate will make the algorithm more responsive. (Default: 0.1)
		PenalizeNewline:  false,
		Seed:             -1, //Sets the random number seed to use for generation. Setting this to a specific number will make the model generate the same text for the same prompt. (Default: 0)

		Runner: Runner{
			// options set when the model is loaded
			NumCtx:             4096, //Sets the size of the context window used to generate the next token. (Default: 2048)
			RopeFrequencyBase:  10000.0,
			RopeFrequencyScale: 1.0,
			NumBatch:           512,
			NumGPU:             -1, //The number of layers to send to the GPU(s). On macOS it defaults to 1 to enable metal support, 0 to disable. // -1 here indicates that NumGPU should be set dynamically
			NumGQA:             1,
			NumThread:          15, //0, // let the runtime decide
			LowVRAM:            false,
			F16KV:              true,
			UseMLock:           false,
			UseMMap:            true,
			UseNUMA:            false,
			EmbeddingOnly:      true,
		},
	}
}
