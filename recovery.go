package csgo

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"web/csgo/cserror"
)

//打印错误发生时的具体信息，包含出现错误的代码行栈帧
func detailMsg(err any) string {
	var pcs [32]uintptr
	//方法代码执行的栈帧控制,并且接收多个栈帧到pcs切片中
	n := runtime.Callers(3, pcs[:])
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%v\n", err))
	for _, pc := range pcs[0:n] {
		fn := runtime.FuncForPC(pc)
		file, line := fn.FileLine(pc)
		sb.WriteString(fmt.Sprintf("\n\t%s:%d", file, line))
	}
	return sb.String()
}

func Recovery(next HandleFunc) HandleFunc {
	return func(ctx *Context) {
		defer func() {
			if err := recover(); err != nil {
				err2 := err.(error)
				if err2 != nil {
					var csError *cserror.CsError
					//如果error是自定义错误 那么进行自定义处理
					if errors.As(err2, &csError) {
						//执行自定义的处理方法
						csError.ExecResult()
						return
					}
				}

				ctx.Logger.Error(detailMsg(err))
				ctx.Fail(http.StatusInternalServerError, "Internal Server Error!!!")
			}
		}()
		next(ctx)
	}
}
