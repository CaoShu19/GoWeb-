package cserror

type CsError struct {
	err       error
	ErrorFunc ErrorFunc
}

func Default() *CsError {
	return &CsError{}
}
func (e *CsError) Error() string {
	return e.err.Error()
}

func (e *CsError) Put(err error) {
	e.check(err)
}

func (e *CsError) check(err error) {
	if err != nil {
		e.err = err
		panic(e)
	}
}

type ErrorFunc func(csError *CsError)

// Result   暴露一个方法，给用户自定义
func (e *CsError) Result(errorFunc ErrorFunc) {
	e.ErrorFunc = errorFunc
}

func (e *CsError) ExecResult() {
	//执行自定义的方法
	e.ErrorFunc(e)
}
