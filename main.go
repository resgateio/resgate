package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/resgateio/resgate/logger"
	"github.com/resgateio/resgate/nats"
	"github.com/resgateio/resgate/server"
)

const (
	// StopTimeout is the duration Resgate waits for all processes to
	// stop before forcefully exiting with an error and a stack trace.
	StopTimeout = 10 * time.Second

	// DefaultNatsURL is the default NATS server to connect to.
	DefaultNatsURL = "nats://127.0.0.1:4222"

	// DefaultRequestTimeout is the timeout duration for NATS requests in milliseconds.
	DefaultRequestTimeout = 3000
)

var usageStr = `
Usage: resgate [options]

Server Options:
    -n, --nats <url>                 NATS Server URL (default: nats://127.0.0.1:4222)
    -i  --addr <host>                Bind to HOST address (default: 0.0.0.0)
    -p, --port <port>                HTTP port for client connections (default: 8080)
    -w, --wspath <path>              WebSocket path for clients (default: /)
    -a, --apipath <path>             Web resource path for clients (default: /api/)
    -r, --reqtimeout <milliseconds>  Timeout duration for NATS requests (default: 3000)
    -m, --metricsport <port>         HTTP port for prometheus metrics connections, disable if not set
    -u, --headauth <method>          Resource method for header authentication
        --apiencoding <type>         Encoding for web resources: json, jsonflat (default: json)
        --putmethod <methodName>     Call method name mapped to HTTP PUT requests
        --deletemethod <methodName>  Call method name mapped to HTTP DELETE requests
        --patchmethod <methodName>   Call method name mapped to HTTP PATCH requests
        --wscompression              Enable WebSocket per message compression
        --resetthrottle <limit>      Limit on parallel requests sent in response to a system reset
        --referencethrottle <limit>  Limit on parallel requests sent when following resource references
    -c, --config <file>              Configuration file

Security Options:
        --tls                        Enable TLS for HTTP (default: false)
        --tlscert <file>             HTTP server certificate file
        --tlskey <file>              Private key for HTTP server certificate
        --creds <file>               NATS User Credentials file
        --natscert <file>            NATS Client certificate file
        --natskey <file>             NATS Client certificate key file
        --natsrootca <file>          NATS Root CA file(s)
        --alloworigin <origin>       Allowed origin(s): *, or <scheme>://<hostname>[:<port>] (default: *)

Logging Options:
    -D, --debug                      Enable debugging output
    -V, --trace                      Enable trace logging
    -DV                              Debug and trace

Common Options:
    -h, --help                       Show this message
    -v, --version                    Show version

Configuration Documentation:         https://resgate.io/docs/get-started/configuration/
`

// Config holds server configuration
type Config struct {
	NatsURL        string   `json:"natsUrl"`
	NatsCreds      string   `json:"natsCreds"`
	NatsTLSCert    string   `json:"natsCert"`
	NatsTLSKey     string   `json:"natsKey"`
	NatsRootCAs    []string `json:"natsRootCAs"`
	RequestTimeout int      `json:"requestTimeout"`
	BufferSize     int      `json:"bufferSize"`
	Debug          bool     `json:"debug"`
	Trace          bool     `json:"trace"`
	server.Config
}

// StringSlice is a slice of strings implementing the flag.Value interface.
type StringSlice []string

func (s *StringSlice) String() string {
	if s == nil {
		return ""
	}
	return strings.Join(*s, ";")
}

// Set adds a value to the slice.
func (s *StringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// SetDefault sets the default values
func (c *Config) SetDefault() {
	if c.NatsURL == "" {
		c.NatsURL = DefaultNatsURL
	}
	if c.RequestTimeout == 0 {
		c.RequestTimeout = DefaultRequestTimeout
	}
	if c.NatsRootCAs == nil {
		c.NatsRootCAs = []string{}
	}
	if c.BufferSize == 0 {
		c.BufferSize = 8192
	}
	c.Config.SetDefault()
}

// Init takes a path to a json encoded file and loads the config
// If no file exists, a new file with default settings is created
func (c *Config) Init(fs *flag.FlagSet, args []string) {
	var (
		showHelp     bool
		showVersion  bool
		configFile   string
		port         uint
		headauth     string
		metricsport  uint
		addr         string
		natsRootCAs  StringSlice
		debugTrace   bool
		allowOrigin  StringSlice
		putMethod    string
		deleteMethod string
		patchMethod  string
	)

	fs.BoolVar(&showHelp, "h", false, "Show this message.")
	fs.BoolVar(&showHelp, "help", false, "Show this message.")
	fs.StringVar(&configFile, "c", "", "Configuration file.")
	fs.StringVar(&configFile, "config", "", "Configuration file.")
	fs.StringVar(&c.NatsURL, "n", "", "NATS Server URL.")
	fs.StringVar(&c.NatsURL, "nats", "", "NATS Server URL.")
	fs.StringVar(&addr, "i", "", "Bind to HOST address.")
	fs.StringVar(&addr, "addr", "", "Bind to HOST address.")
	fs.UintVar(&port, "p", 0, "HTTP port for client connections.")
	fs.UintVar(&port, "port", 0, "HTTP port for client connections.")
	fs.StringVar(&c.WSPath, "w", "", "WebSocket path for clients.")
	fs.StringVar(&c.WSPath, "wspath", "", "WebSocket path for clients.")
	fs.StringVar(&c.APIPath, "a", "", "Web resource path for clients.")
	fs.StringVar(&c.APIPath, "apipath", "", "Web resource path for clients.")
	fs.StringVar(&headauth, "u", "", "Resource method for header authentication.")
	fs.StringVar(&headauth, "headauth", "", "Resource method for header authentication.")
	fs.UintVar(&metricsport, "m", 0, "HTTP port for prometheus metrics connections, disable if not set.")
	fs.UintVar(&metricsport, "metricsport", 0, "HTTP port for prometheus metrics connections, disable if not set.")
	fs.BoolVar(&c.TLS, "tls", false, "Enable TLS for HTTP.")
	fs.StringVar(&c.TLSCert, "tlscert", "", "HTTP server certificate file.")
	fs.StringVar(&c.TLSKey, "tlskey", "", "Private key for HTTP server certificate.")
	fs.StringVar(&c.APIEncoding, "apiencoding", "", "Encoding for web resources.")
	fs.IntVar(&c.RequestTimeout, "r", 0, "Timeout in milliseconds for NATS requests.")
	fs.IntVar(&c.RequestTimeout, "reqtimeout", 0, "Timeout in milliseconds for NATS requests.")
	fs.StringVar(&c.NatsCreds, "creds", "", "NATS User Credentials file.")
	fs.StringVar(&c.NatsTLSCert, "natscert", "", "NATS Client certificate file.")
	fs.StringVar(&c.NatsTLSKey, "natskey", "", "NATS Client certificate key file.")
	fs.Var(&natsRootCAs, "natsrootca", "NATS Root CA file(s).")
	fs.Var(&allowOrigin, "alloworigin", "Allowed origin(s) for CORS.")
	fs.StringVar(&putMethod, "putmethod", "", "Call method name mapped to HTTP PUT requests.")
	fs.StringVar(&deleteMethod, "deletemethod", "", "Call method name mapped to HTTP DELETE requests.")
	fs.StringVar(&patchMethod, "patchmethod", "", "Call method name mapped to HTTP PATCH requests.")
	fs.BoolVar(&c.WSCompression, "wscompression", false, "Enable WebSocket per message compression.")
	fs.IntVar(&c.ResetThrottle, "resetthrottle", 0, "Limit on parallel requests sent in response to a system reset.")
	fs.IntVar(&c.ReferenceThrottle, "referencethrottle", 0, "Limit on parallel requests sent when following resource references.")
	fs.BoolVar(&c.Debug, "D", false, "Enable debugging output.")
	fs.BoolVar(&c.Debug, "debug", false, "Enable debugging output.")
	fs.BoolVar(&c.Trace, "V", false, "Enable trace logging.")
	fs.BoolVar(&c.Trace, "trace", false, "Enable trace logging.")
	fs.BoolVar(&debugTrace, "DV", false, "Enable debug and trace logging.")
	fs.BoolVar(&showVersion, "version", false, "Print version information.")
	fs.BoolVar(&showVersion, "v", false, "Print version information.")

	if err := fs.Parse(args); err != nil {
		printAndDie(fmt.Sprintf("Error parsing command arguments: %s", err.Error()), true)
	}

	if port >= 1<<16 {
		printAndDie(fmt.Sprintf(`Invalid port "%d": must be less than 65536`, port), true)
	}

	if metricsport != 0 {
		if metricsport >= 1<<16 {
			printAndDie(fmt.Sprintf(`Invalid metrics port "%d": must be less than 65536`, metricsport), true)
		}
		if metricsport == port {
			printAndDie(fmt.Sprintf(`Invalid metrics port "%d": must be different from API port ("%d")`, metricsport, port), true)
		}
	}

	if showHelp {
		usage()
	}

	if showVersion {
		version()
	}

	writeConfig := false
	if configFile != "" {
		fin, err := ioutil.ReadFile(configFile)
		if err != nil {
			if !os.IsNotExist(err) {
				printAndDie(fmt.Sprintf("Error loading config file: %s", err), false)
			}

			c.SetDefault()
			writeConfig = true
		} else {
			err = json.Unmarshal(fin, c)
			if err != nil {
				printAndDie(fmt.Sprintf("Error parsing config file: %s", err), false)
			}

			// Overwrite configFile options with command line options
			fs.Parse(args)
		}
	}

	if port > 0 {
		c.Port = uint16(port)
	}
	if metricsport > 0 {
		c.MetricsPort = uint16(metricsport)
	}

	// Helper function to set string pointers to nil if empty.
	setString := func(v string, s **string) {
		if v == "" {
			*s = nil
		} else {
			*s = &v
		}
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "u":
			fallthrough
		case "headauth":
			setString(headauth, &c.HeaderAuth)
		case "natsrootca":
			c.NatsRootCAs = natsRootCAs
		case "alloworigin":
			str := allowOrigin.String()
			c.AllowOrigin = &str
		case "putmethod":
			setString(putMethod, &c.PUTMethod)
		case "deletemethod":
			setString(deleteMethod, &c.DELETEMethod)
		case "patchmethod":
			setString(patchMethod, &c.PATCHMethod)
		case "i":
			fallthrough
		case "addr":
			c.Addr = &addr
		case "DV":
			c.Debug = true
			c.Trace = true
		}
	})

	// Any value not set, set it now
	c.SetDefault()

	// Write config file
	if writeConfig {
		fout, err := json.MarshalIndent(c, "", "\t")
		if err != nil {
			printAndDie(fmt.Sprintf("Error encoding config: %s", err), false)
		}
		ioutil.WriteFile(configFile, fout, os.FileMode(0664))
	}
}

// usage will print out the flag options for the server.
func usage() {
	fmt.Printf("%s\n", usageStr)
	os.Exit(0)
}

// version will print out the current resgate and protocol version.
func version() {
	fmt.Printf("resgate  v%s\nprotocol v%s\n", server.Version, server.ProtocolVersion)
	os.Exit(0)
}

func printAndDie(msg string, showUsage bool) {
	fmt.Fprintln(os.Stderr, msg)
	if showUsage {
		fmt.Fprintln(os.Stderr, usageStr)
	}
	os.Exit(1)
}

func main() {
	fs := flag.NewFlagSet("resgate", flag.ExitOnError)
	fs.Usage = usage

	var cfg Config

	cfg.Init(fs, os.Args[1:])

	l := logger.NewStdLogger(cfg.Debug, cfg.Trace)

	// Remove below if clause after release of version >= 1.3.x
	if cfg.RequestTimeout <= 10 {
		fmt.Fprintf(os.Stderr, "[DEPRECATED] Request timeout should be in milliseconds.\nChange your requestTimeout from %d to %d, and you won't be bothered anymore.\n", cfg.RequestTimeout, cfg.RequestTimeout*1000)
		cfg.RequestTimeout *= 1000
	}
	serv, err := server.NewService(&nats.Client{
		URL:            cfg.NatsURL,
		Creds:          cfg.NatsCreds,
		ClientCert:     cfg.NatsTLSCert,
		ClientKey:      cfg.NatsTLSKey,
		RootCAs:        cfg.NatsRootCAs,
		RequestTimeout: time.Duration(cfg.RequestTimeout) * time.Millisecond,
		BufferSize:     cfg.BufferSize,
		Logger:         l,
	}, cfg.Config)
	if err != nil {
		printAndDie(fmt.Sprintf("Failed to initialize server: %s", err.Error()), false)
	}
	serv.SetLogger(l)

	if err := serv.Start(); err != nil {
		printAndDie(fmt.Sprintf("Failed to start server: %s", err.Error()), false)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	select {
	case <-stop:
	case err := <-serv.StopChannel():
		if err != nil {
			printAndDie(fmt.Sprintf("Server stopped with an error: %s", err.Error()), false)
		}
	}
	// Await for waitGroup to be done
	done := make(chan struct{})
	go func() {
		defer close(done)
		serv.Stop(nil)
	}()

	select {
	case <-done:
	case <-time.After(StopTimeout):
		panic("Shutdown timed out")
	}
}
