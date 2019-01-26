package toolbelt

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/y-a-r-g/json2flag"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/netutil"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"reflect"
	"time"
)

var TGin = reflect.TypeOf(GinTool{})

type GinTool struct {
	*gin.Engine
	config     *GinConfig
	httpServer *http.Server
}

type GinConfig struct {
	Debug       bool
	HttpAddr    string
	HttpsAddr   string
	HttpsDomain string
	Timeout     time.Duration
	MaxClients  uint
}

func (t *GinTool) Configure(config ...interface{}) {
	t.config = &GinConfig{
		HttpAddr:  ":http",
		HttpsAddr: ":https",
		Timeout:   1 * time.Minute,
	}

	if len(config) > 0 {
		*t.config = *config[0].(*GinConfig)
	}

	json2flag.FlagPrefixed(t.config, map[string]string{
		"Debug":       "run in debug mode",
		"HttpAddr":    "http address to listen",
		"HttpsAddr":   "https address to listen",
		"HttpsDomain": "domain to fetch certificate to",
		"Timeout":     "request timeout",
		"MaxClients":  "maximum simultaneous clients",
	}, TGin.Name())
}

func (t *GinTool) Dependencies() []reflect.Type {
	return []reflect.Type{TLogrus}
}

func (t *GinTool) Start(belt IBelt) {
	log := belt.Tool(TLogrus).(*LogrusTool)

	t.httpServer = &http.Server{
		Addr:           t.config.HttpAddr,
		Handler:        t,
		ReadTimeout:    t.config.Timeout,
		WriteTimeout:   t.config.Timeout,
		MaxHeaderBytes: 1 << 15,
	}

	if t.config.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	t.Engine = gin.New()
	t.Use(requestLogger(log.Logger, t.config.Debug))

	t.Use(gin.RecoveryWithWriter(log.WriterLevel(logrus.WarnLevel))) //TODO: make buffered writer - this one will spam to email

	if t.config.HttpAddr != "" {
		httpListener, err := net.Listen("tcp", t.config.HttpAddr)
		if err != nil {
			panic(err)
		}

		httpListener = tcpKeepAliveListener{
			TCPListener:     httpListener.(*net.TCPListener),
			KeepAlivePeriod: t.config.Timeout,
			log:             log,
		}

		if t.config.MaxClients > 0 {
			httpListener = netutil.LimitListener(httpListener, int(t.config.MaxClients))
		}

		go func() {
			defer func() {
				err := httpListener.Close()
				if err != nil {
					log.WithError(err).Error("cannot close listener")
				}
			}()
			err := t.httpServer.Serve(httpListener)

			if err != http.ErrServerClosed && err != nil {
				log.WithError(err).Error("http server stopped")
			}
		}()
	}

	if t.config.HttpsAddr != "" && t.config.HttpsDomain != "" {
		httpsListener := autocert.NewListener(t.config.HttpsDomain)

		go func() {
			defer func() {
				err := httpsListener.Close()
				if err != nil {
					log.WithError(err).Error("cannot close listener")
				}
			}()
			err := t.httpServer.Serve(httpsListener)
			if err != http.ErrServerClosed && err != nil {
				log.WithError(err).Error("https server stopped")
			}
		}()
	}
}

func (t *GinTool) Stop(belt IBelt) {
	err := t.httpServer.Shutdown(context.Background())
	if err != nil {
		log := belt.Tool(TLogrus).(*LogrusTool)
		log.WithError(err).Fatal("error while shutting server down")
	}
}

type tcpKeepAliveListener struct {
	*net.TCPListener
	KeepAlivePeriod time.Duration
	log             *LogrusTool
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	if ln.KeepAlivePeriod == 0 {
		err := tc.SetKeepAlive(false)
		if err != nil {
			ln.log.WithError(err).Warn("failed to switch off keep alive")
		}
	} else {
		err := tc.SetKeepAlive(true)
		if err != nil {
			ln.log.WithError(err).Warn("failed to switch on keep alive")
		}
		err = tc.SetKeepAlivePeriod(ln.KeepAlivePeriod)
		if err != nil {
			ln.log.WithError(err).Warn("failed to set keep alive period")
		}
	}
	return tc, nil
}

func requestLogger(log *logrus.Logger, debug bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var start time.Time
		var blw *bodyLogWriter
		var originalBody []byte
		path := c.Request.URL.Path

		if debug {
			start = time.Now()
			var err error
			originalBody, err = ioutil.ReadAll(c.Request.Body)
			if err != nil {
				_ = c.Error(err)
			}

			c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(originalBody))

			blw = &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
			c.Writer = blw
		}

		c.Next()

		statusCode := c.Writer.Status()

		if (len(c.Errors) > 0) || statusCode >= 500 || debug {
			stop := time.Since(start)
			clientIP := c.ClientIP()
			latency := int(math.Ceil(float64(stop.Nanoseconds()) / 1000000.0))

			reqHeaders, _ := json.Marshal(c.Request.Header)
			resHeaders, _ := json.Marshal(c.Writer.Header())

			entry := logrus.NewEntry(log).WithFields(logrus.Fields{
				"path":             path,
				"request":          string(originalBody),
				"response":         blw.body.String(),
				"ip":               clientIP,
				"latency":          latency,
				"method":           c.Request.Method,
				"status":           statusCode,
				"request-headers":  string(reqHeaders),
				"response-headers": string(resHeaders),
			})

			if len(c.Errors) > 0 {
				entry.Error(c.Errors.ByType(gin.ErrorTypePrivate).String())
			} else {
				if statusCode >= 500 {
					entry.Error("server error")
				} else if statusCode >= 400 {
					entry.Warn("client error")
				} else {
					entry.Info("request ok")
				}
			}
		}
	}
}

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}
