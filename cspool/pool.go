package cspool

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"web/csgo/config"
)

type sig struct {
}

const DefaultExpire = 3

var (
	ErrorInValidCap    = errors.New("cap can not less than 0")
	ErrorInValidExpire = errors.New("expire time can not less than 0")
	ErrorHasClosed     = errors.New("pool has been release")
)

type Pool struct {
	//max协程数目
	cap int32
	//正在运行的协程的数目
	running int32
	//核心协程，空闲的协程
	Workers []*Worker
	//空闲的worker的生存时间
	expire time.Duration
	//关闭资源的信号，pool的状态
	release chan sig
	//协程锁
	lock sync.Mutex
	//once 保证release只能调用一次
	once sync.Once
	//存放协程的缓存
	workerCache sync.Pool
	//条件变量：协调协程
	cond *sync.Cond
	//PanicHandler
	PanicHandler func()
}

func NewPool(cap int) (*Pool, error) {
	return NewTimePool(cap, DefaultExpire)
}
func NewPoolConf() (*Pool, error) {
	cap, ok := config.Conf.Pool["cap"]
	if !ok {
		return nil, errors.New("cap config not exist")
	}

	return NewTimePool(int(cap.(int64)), DefaultExpire)
}
func NewTimePool(cap int, expire int) (*Pool, error) {
	if cap <= 0 {
		return nil, ErrorInValidCap
	}
	if expire <= 0 {
		return nil, ErrorInValidExpire
	}

	p := &Pool{
		cap:     int32(cap),
		expire:  time.Duration(expire) * time.Second,
		release: make(chan sig, 1),
	}
	p.workerCache.New = func() any {
		return &Worker{
			pool: p,
			task: make(chan func(), 1),
		}
	}
	//条件变量的锁，我们默认使用互斥锁
	p.cond = sync.NewCond(&p.lock)
	//开启定时清除
	go p.expireWorker()
	return p, nil
}

//定时清理过期的worker
func (p *Pool) expireWorker() {
	ticker := time.NewTicker(p.expire)
	for range ticker.C {
		//定时循环空闲的worker，如果当前时间和worker的最后运行任务的时间 差值大于expire 进行清理
		if p.IsClosed() {
			break
		}
		p.lock.Lock()

		idleWorkers := p.Workers
		n := len(idleWorkers) - 1
		//fmt.Println("定时任务：", n)
		if n >= 0 {
			var clearN = -1
			for i, w := range idleWorkers {
				if time.Now().Sub(w.lastTime) <= p.expire {
					//前面都不满足，后面肯定都不满住
					break
				}
				clearN = i
				w.task <- nil
			}
			if clearN != -1 {
				if clearN >= len(idleWorkers)-1 {
					p.Workers = idleWorkers[:0]
				} else {
					p.Workers = idleWorkers[clearN+1:]
				}
				fmt.Printf("任务清除完成，running：%d,workers:%v \n", p.running, p.Workers)
			}
		}
		p.lock.Unlock()
	}
}

func (p *Pool) Submit(task func()) error {
	if len(p.release) > 0 {
		return ErrorHasClosed
	}
	//获取池中任务，执行任务即可
	w := p.GetWork()
	//将任务放到任务队列
	w.task <- task
	//计数加一
	w.pool.incRunning()

	return nil
}

func (p *Pool) GetWork() *Worker {
	// 获取pool中的协程
	// 如果有空闲work那么直接获取
	p.lock.Lock()
	idleWorkers := p.Workers
	n := len(idleWorkers) - 1
	//有空闲worker
	if n >= 0 {

		w := idleWorkers[n]
		idleWorkers[n] = nil
		p.Workers = idleWorkers[0:n]
		p.lock.Unlock()
		return w
	}
	// 如果没有空闲worker那么新建一个worker,当然正在运行的worker+空闲worker 的数目大于maxCap容量
	if p.running <= p.cap {
		//正在运行的协程数目是小于最大容量的，直接新创建一个
		p.lock.Unlock()
		//从缓存池中拿到
		c := p.workerCache.Get()
		var w *Worker
		if c == nil {
			w = &Worker{
				pool: p,
				task: make(chan func(), 1),
			}
		} else {
			w = c.(*Worker)
		}
		//让协程执行任务
		w.run()
		return w
	}
	p.lock.Unlock()
	//若是大于最大容量后,阻塞等待,直到有空闲协程
	return p.waitIdleWorker()

}

//任务阻塞，因为可用协程不够，需要等待任务执行完毕
func (p *Pool) waitIdleWorker() *Worker {
	p.lock.Lock()
	p.cond.Wait()

	idleWorkers := p.Workers
	n := len(idleWorkers) - 1
	//如果空闲队列长度为0 空闲协程为0
	if n < 0 {
		p.lock.Unlock()
		if p.running <= p.cap {
			//正在运行的协程数目是小于最大容量的，直接新创建一个
			//从缓存池中拿到
			c := p.workerCache.Get()
			var w *Worker
			if c == nil {
				w = &Worker{
					pool: p,
					task: make(chan func(), 1),
				}
			} else {
				w = c.(*Worker)
			}
			//让协程执行任务
			w.run()
			return w
		}
		return p.waitIdleWorker()
	}
	w := idleWorkers[n]
	idleWorkers[n] = nil
	p.Workers = idleWorkers[0:n]
	p.lock.Unlock()
	return w
}

func (p *Pool) incRunning() {
	//正在运行的协程加1，用原子类
	atomic.AddInt32(&p.running, 1)
}

// PutWorker 协程复用
func (p *Pool) PutWorker(w *Worker) {
	w.lastTime = time.Now()
	p.lock.Lock()
	//将协程添加到池中的workerS
	p.Workers = append(p.Workers, w)
	//放入新的空闲协程，此协程可处理任务，通知可以有可用协程去执行任务
	p.cond.Signal()
	p.lock.Unlock()
}

func (p *Pool) decRunning() {
	//正在运行的协程加1，用原子类
	atomic.AddInt32(&p.running, -1)
}

func (p *Pool) Release() {
	p.once.Do(func() {
		p.lock.Lock()

		workers := p.Workers
		for i, w := range workers {
			w.task = nil
			w.pool = nil
			workers[i] = nil
		}
		p.Workers = nil
		p.lock.Unlock()
		p.release <- sig{}
	})
}

func (p *Pool) IsClosed() bool {
	return len(p.release) > 0
}

func (p *Pool) Restart() bool {
	if p.IsClosed() {
		return true
	}

	//将关闭状态修改为空
	_ = <-p.release
	//开启定时清除
	go p.expireWorker()
	return true
}
