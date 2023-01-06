package cspool

import (
	"time"
	csLog "web/csgo/log"
)

type Worker struct {
	//向协程发送执行要任务 -> 任务队列
	task chan func()
	//属于那个池
	pool *Pool
	//last time 执行任务的最后时间
	lastTime time.Time
}

func (w *Worker) run() {
	//开协程执行任务
	go w.running()
}

func (w *Worker) running() {

	defer func() {
		w.pool.decRunning()
		w.pool.workerCache.Put(w)
		//捕获异常
		if err := recover(); err != nil {
			//处理异常
			if w.pool.PanicHandler != nil {
				w.pool.waitIdleWorker()
			} else {
				csLog.Default().Error(err)
			}
		}
		w.pool.cond.Signal()
	}()
	//循环执行运行队列中的任务
	for f := range w.task {
		if f == nil {
			//如果为执行的任务为空，那么就放回缓存
			w.pool.workerCache.Put(w)
			return
		}
		f()
		//任务运行完成，就让work空闲
		w.pool.PutWorker(w)
	}

}
