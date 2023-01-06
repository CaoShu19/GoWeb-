package csgo

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	greenBg   = "\033[97;42m"
	whiteBg   = "\033[90;47m"
	yellowBg  = "\033[90;43m"
	redBg     = "\033[97;41m"
	blueBg    = "\033[97;44m"
	magentaBg = "\033[97;45m"
	cyanBg    = "\033[97;46m"
	green     = "\033[32m"
	white     = "\033[37m"
	yellow    = "\033[33m"
	red       = "\033[31m"
	blue      = "\033[34m"
	magenta   = "\033[35m"
	cyan      = "\033[36m"
	reset     = "\033[0m"
)

// DefaultWriter  is stander output
var DefaultWriter io.Writer = os.Stdout

type LoggerFormatter = func(params *LogFormatterParams) string

// LogFormatterParams log_information
type LogFormatterParams struct {
	Request        *http.Request
	TimeStamp      time.Time
	StatusCode     int
	Latency        time.Duration
	ClientIP       net.IP
	Method         string
	Path           string
	IsDisplayColor bool
}

func (p LogFormatterParams) StatusCodeColor() interface{} {
	code := p.StatusCode
	switch code {
	case http.StatusOK:
		return green
	default:
		return red
	}
}

func (p LogFormatterParams) ResetColor() interface{} {
	return reset
}

type LoggingConfig struct {
	Formatter LoggerFormatter
	out       io.Writer
}

var defaultFormatter = func(params *LogFormatterParams) string {
	var statusCodeColor = params.StatusCodeColor()
	var resetColor = params.ResetColor()
	if params.Latency > time.Minute {
		params.Latency = params.Latency.Truncate(time.Second)
	}

	if !params.IsDisplayColor {
		// do not show color for linux by default
		return fmt.Sprintf("[msgo] %v |  %3d  | %13v | %15s |%-7s %#v",
			params.TimeStamp.Format("2006/01/02 - 15:04:05"),
			params.StatusCode,
			params.Latency, params.ClientIP, params.Method, params.Path,
		)
	}
	return fmt.Sprintf("%s [msgo] %s |%s %v %s| %s %3d %s |%s %13v %s| %15s  |%s %-7s %s %s %#v %s\n",
		yellow, resetColor, blue, params.TimeStamp.Format("2006/01/02 - 15:04:05"), resetColor,
		statusCodeColor, params.StatusCode, resetColor,
		red, params.Latency, resetColor,
		params.ClientIP,
		magenta, params.Method, resetColor,
		cyan, params.Path, resetColor,
	)
}

func LoggingWithConfig(conf LoggingConfig, next HandleFunc) HandleFunc {
	formatter := conf.Formatter
	if formatter == nil {
		formatter = defaultFormatter
	}
	out := conf.out
	displayColor := false
	if out == nil {
		out = DefaultWriter
		displayColor = true
	}

	return func(ctx *Context) {
		param := &LogFormatterParams{
			IsDisplayColor: displayColor,
		}

		log.Println("log....")
		// Start timer
		start := time.Now()
		path := ctx.R.URL.Path
		raw := ctx.R.URL.RawQuery
		//执行业务
		next(ctx)
		// stop timer
		stop := time.Now()
		latency := stop.Sub(start)
		ip, _, _ := net.SplitHostPort(strings.TrimSpace(ctx.R.RemoteAddr))
		clientIP := net.ParseIP(ip)
		method := ctx.R.Method
		statusCode := ctx.StatusCode

		if raw != "" {
			path = path + "?" + raw
		}

		param.Request = ctx.R
		param.ClientIP = clientIP
		param.TimeStamp = stop
		param.Latency = latency
		param.Path = path
		param.Method = method
		param.StatusCode = statusCode

		fmt.Fprint(out, formatter(param))
	}
}

func Logging(next HandleFunc) HandleFunc {
	return LoggingWithConfig(LoggingConfig{}, next)
}
