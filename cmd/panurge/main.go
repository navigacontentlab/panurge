package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/navigacontentlab/panurge/v2/navigaid"
	"github.com/urfave/cli/v2"
)

func main() {
	app := NewCLIApplication()

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%v", err.Error()) //nolint:forbidigo
		os.Exit(1)
	}
}

func NewCLIApplication() cli.App {
	return cli.App{
		Commands: []*cli.Command{
			{
				Name:        "navigaid-mock",
				Action:      navigaIDMock,
				Description: "runs a NavigaID mock server",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "addr",
						Value: ":1066",
					},
					&cli.PathFlag{
						Name: "config",
					},
				},
			},
		},
	}
}

func navigaIDMock(c *cli.Context) error {
	addr := c.String("addr")
	confPath := c.Path("config")

	opts := navigaid.MockServerOptions{
		Claims: navigaid.Claims{
			Org: "testorg",
		},
		TTL: 600,
	}

	if confPath != "" {
		conf, err := os.ReadFile(confPath)
		if err != nil {
			return fmt.Errorf("failed to read config: %w", err)
		}

		err = json.Unmarshal(conf, &opts)
		if err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}
	}

	mockService, err := navigaid.NewMockService(opts)

	if err != nil {
		return fmt.Errorf("failed to create mock service: %w", err)
	}

	var server http.Server

	server.Addr = addr
	server.Handler = mockService

	err = server.ListenAndServe()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
