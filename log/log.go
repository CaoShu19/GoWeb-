package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"time"
	"web/csgo/internal/csstrings"
)

type LoggerLevel int

func (l LoggerLevel) Level() any {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO "
	case LevelError:
		return "ERROR"
	}
	return nil
}

const (
	// LevelDebug 为什么要用iota？？？
	LevelDebug LoggerLevel = iota
	LevelInfo
	LevelError
)

// Fields 字段类型
type Fields map[string]any

type LoggingFormatter interface {
	Format(param *LoggingFormatParam) string
}
type LoggingFormatParam struct {
	IsColor      bool
	Level        LoggerLevel
	LoggerFields Fields
	Msg          any
}
type LoggerFormatter struct {
	IsColor      bool
	Level        LoggerLevel
	LoggerFields Fields
}

// Logger 日志
type Logger struct {
	//级别信息
	Level LoggerLevel
	//输出通道
	Outs []*LoggerWriter
	//输出模式
	Formatter LoggingFormatter
	//日志字段
	LoggerFields Fields
	//日志文件路径
	logPath string
	//用户设置文件日志文件大小
	LogFileSize int64
}

type LoggerWriter struct {
	Level LoggerLevel
	Out   io.Writer
}

func New() *Logger {
	return &Logger{}
}

func Default() *Logger {
	logger := New()
	logger.Level = LevelDebug
	w := &LoggerWriter{
		Level: LevelDebug,
		Out:   os.Stdout,
	}
	logger.Outs = append(logger.Outs, w)
	logger.Formatter = &TextFormatter{}
	logger.logPath = "./log"
	return logger
}

// Debug 开发阶段常用
func (l *Logger) Debug(msg any) {
	l.Print(LevelDebug, msg)
}
func (l *Logger) Error(msg any) {
	l.Print(LevelError, msg)
}

// Info 部署上线阶段
func (l *Logger) Info(msg any) {
	l.Print(LevelInfo, msg)
}

// Print 将日志打印到输出端中
func (l *Logger) Print(level LoggerLevel, msg any) {
	if l.Level > level {
		//当前级别大于输入级别 不打印对应级别的日志
		return
	}
	param := &LoggingFormatParam{
		Level:        level,
		LoggerFields: l.LoggerFields,
		Msg:          msg,
	}

	str := l.Formatter.Format(param)
	for _, out := range l.Outs {
		if out.Out == os.Stdout {
			param.IsColor = true
			str = l.Formatter.Format(param)
			fmt.Fprintln(out.Out, str)
		}
		if (out.Level == -1 || level == out.Level) && out.Out != os.Stdout {
			fmt.Fprintln(out.Out, str)
			l.CheckFileSize(out)
		}
	}
}
func (l *Logger) WithFields(fields Fields) *Logger {
	return &Logger{
		Formatter:    l.Formatter,
		Outs:         l.Outs,
		Level:        l.Level,
		LoggerFields: fields,
	}
}

func (l *Logger) SetLogPath(logPath string) {
	l.logPath = logPath
	l.Outs = append(l.Outs, &LoggerWriter{Level: -1, Out: FileWriter(path.Join(logPath, "all.log"))})
	l.Outs = append(l.Outs, &LoggerWriter{Level: 0, Out: FileWriter(path.Join(logPath, "debug.log"))})
	l.Outs = append(l.Outs, &LoggerWriter{Level: 1, Out: FileWriter(path.Join(logPath, "info.log"))})
	l.Outs = append(l.Outs, &LoggerWriter{Level: 2, Out: FileWriter(path.Join(logPath, "error.log"))})
}

// CheckFileSize 判断文件大小
func (l *Logger) CheckFileSize(w *LoggerWriter) {
	//获得文件输出端
	logFile := w.Out.(*os.File)
	if logFile != nil {
		//获得文件状态
		stat, err := logFile.Stat()
		if err != nil {
			log.Println(err)
			return
		}
		//文件大小获得
		size := stat.Size()
		if l.LogFileSize <= 0 {
			//默认设置日志文件大小为100MB
			l.LogFileSize = 100 << 20
		}
		// 如果超过设置的大小，重新新建一个日志文件
		if size >= l.LogFileSize {
			_, name := path.Split(logFile.Name())
			fileName := name[0:strings.Index(name, ".")]

			writer := FileWriter(path.Join(l.logPath, csstrings.JoinStrings(fileName, ".", time.Now().UnixMilli(), ".log")))
			w.Out = writer
		}
	}

}

//默认被弃用的日志输出格式
func (f *LoggerFormatter) formatter(msg any) string {
	now := time.Now()
	if f.IsColor {
		//要带颜色  error的颜色 为红色 info为绿色 debug为蓝色
		levelColor := f.LevelColor()
		msgColor := f.MsgColor()
		return fmt.Sprintf("%s [csgo] %s %s%v%s | level= %s %s %s | msg=%s %#v %s | fields = %#v \n",
			yellow, reset, blue, now.Format("2006/01/02 - 15:04:05"), reset,
			levelColor, f.Level.Level(), reset, msgColor, msg, reset, f.LoggerFields,
		)
	}
	return fmt.Sprintf("[csgo] %v | level=%s | msg=%#v | fields = %#v \n",
		now.Format("2006/01/02 - 15:04:05"),
		f.Level.Level(), msg, f.LoggerFields,
	)
}

func (f *LoggerFormatter) LevelColor() string {
	switch f.Level {
	case LevelDebug:
		return blue
	case LevelInfo:
		return green
	case LevelError:
		return red
	default:
		return cyan
	}
}

func (f *LoggerFormatter) MsgColor() interface{} {
	switch f.Level {
	case LevelDebug:
		return ""
	case LevelInfo:
		return ""
	case LevelError:
		return red
	default:
		return cyan
	}
}

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

func FileWriter(name string) io.Writer {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	return file
}
