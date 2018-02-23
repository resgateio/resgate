package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jirenius/resgate/mq/nats"
	"github.com/jirenius/resgate/service"
)

var (
	configFile = flag.String("conf", "config.json", "File path to configuration-file")
)

var stopTimeout = 10 * time.Second

// Config holds server configuration
type Config struct {
	NatsURL string `json:"natsUrl"`
	Debug   bool   `json:"debug,omitempty"`
	service.Config
}

// SetDefault sets the default values
func (c *Config) SetDefault() {
	c.NatsURL = "nats://127.0.0.1:4222"
	c.Config.SetDefault()
}

// Init takes a path to a json encoded file and loads the config
// If no file exists, a new file with default settings is created
func (c *Config) Init(file string) error {
	fin, err := ioutil.ReadFile(file)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		c.SetDefault()

		fout, err := json.MarshalIndent(c, "", "\t")
		if err != nil {
			return err
		}

		ioutil.WriteFile(file, fout, os.FileMode(0664))
	} else {
		err = json.Unmarshal(fin, c)
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	flag.Parse()

	var cfg Config
	cfg.Init(*configFile)

	nats.SetDebug(cfg.Debug)
	service.SetDebug(cfg.Debug)
	serv := service.NewService(&nats.Client{URL: cfg.NatsURL, RequestTimeout: time.Second * 5}, cfg.Config)

	if err := serv.Start(); err != nil {
		os.Exit(1)
	}

	stop := make(chan os.Signal)
	signal.Notify(stop,
		os.Interrupt,
		os.Kill,
		syscall.SIGHUP,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	select {
	case <-stop:
	case <-serv.StopChannel():
	}
	// Await for waitGroup to be done
	done := make(chan struct{})
	go func() {
		defer close(done)
		serv.Stop()
	}()

	select {
	case <-done:
	case <-time.After(stopTimeout):
		panic("Shutdown timed out")
	}
}
