package cohere

import (
	"context"
	"fmt"
	"os"

	coherego "github.com/henomis/cohere-go"
	"github.com/henomis/cohere-go/model"
	"github.com/henomis/cohere-go/request"
	"github.com/henomis/cohere-go/response"
	"github.com/henomis/lingoose/chat"
)

type Model model.Model

const (
	ModelCommand             Model = Model(model.ModelCommand)
	ModelCommandNightly      Model = Model(model.ModelCommandNightly)
	ModelCommandLight        Model = Model(model.ModelCommandLight)
	ModelCommandLightNightly Model = Model(model.ModelCommandLightNightly)
)

const (
	DefaultMaxTokens   = 20
	DefaultTemperature = 0.75
	DefaultModel       = ModelCommand
)

type Cohere struct {
	client      *coherego.Client
	model       Model
	temperature float64
	maxTokens   int
	verbose     bool
	stop        []string
}

func NewCompletion() *Cohere {
	return &Cohere{
		client:      coherego.New(os.Getenv("COHERE_API_KEY")),
		model:       DefaultModel,
		temperature: DefaultTemperature,
		maxTokens:   DefaultMaxTokens,
	}
}

func (c *Cohere) WithModel(model Model) *Cohere {
	c.model = model
	return c
}

func (c *Cohere) WithTemperature(temperature float64) *Cohere {
	c.temperature = temperature
	return c
}

func (c *Cohere) WithMaxTokens(maxTokens int) *Cohere {
	c.maxTokens = maxTokens
	return c
}

func (c *Cohere) WithAPIKey(apiKey string) *Cohere {
	c.client = coherego.New(apiKey)
	return c
}

func (c *Cohere) WithVerbose(verbose bool) *Cohere {
	c.verbose = verbose
	return c
}

func (o *Cohere) WithStop(stop []string) *Cohere {
	o.stop = stop
	return o
}

func (c *Cohere) Completion(ctx context.Context, prompt string) (string, error) {

	resp := &response.Generate{}
	err := c.client.Generate(
		context.Background(),
		&request.Generate{
			Prompt:        prompt,
			Temperature:   &c.temperature,
			MaxTokens:     &c.maxTokens,
			Model:         (*model.Model)(&c.model),
			StopSequences: c.stop,
		},
		resp,
	)
	if err != nil {
		return "", err
	}

	if len(resp.Generations) == 0 {
		return "", fmt.Errorf("no generations returned")
	}

	output := resp.Generations[0].Text

	if c.verbose {
		fmt.Printf("---USER---\n%s\n", prompt)
		fmt.Printf("---AI---\n%s\n", output)
	}

	return output, nil
}

func (c *Cohere) Chat(ctx context.Context, prompt *chat.Chat) (string, error) {
	return "", fmt.Errorf("not implemented")
}
