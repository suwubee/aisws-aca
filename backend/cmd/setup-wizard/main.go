package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ai-coding-assistant/service/setupwizard"
)

func main() {
	var projectRoot string
	var host string
	var port int

	flag.StringVar(&projectRoot, "project-root", "", "Project root (auto-detect if empty)")
	flag.StringVar(&host, "host", "0.0.0.0", "Bind host for setup wizard")
	flag.IntVar(&port, "port", 0, "Bind port for setup wizard (0=random)")
	flag.Parse()

	if err := setupwizard.Run(setupwizard.RunOptions{
		ProjectRoot: projectRoot,
		BindHost:    host,
		Port:        port,
	}); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

