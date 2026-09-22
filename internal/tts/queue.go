// Package tts 实现 TTS 任务的优先级队列：固定数量 worker 并发执行，
// admin 任务插队（排在所有等待中的普通任务之前，但不打断正在执行的任务）。
// 任务异步执行，客户端提交后轮询状态，与 HTTP 请求生命周期解耦。
package tts

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Status string

const (
	StatusQueued  Status = "queued"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

const (
	maxWaiting  = 20                // 排队上限（adminQ + normalQ 等待总数）
	queueTTL    = 15 * time.Minute  // 排队超时：超过则自动放弃
	jobTTL      = 15 * time.Minute  // done/failed 条目保留时间
	runTimeout  = 120 * time.Second // 单次合成执行超时
	sweepPeriod = time.Minute
)

// ErrQueueFull 表示等待队列已满，handler 层映射为 429。
var ErrQueueFull = errors.New("queue is full")

type Job struct {
	ID        string    `json:"id"`
	Status    Status    `json:"status"`
	Err       string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`

	Owner   string `json:"-"` // 提交者，用于 GET/DELETE 鉴权
	Admin   bool   `json:"-"`
	Text    string `json:"-"` // 执行时才写临时文件，避免排队超时留下孤儿 txt
	Voice   string `json:"-"`
	Rate    int    `json:"-"`
	Pitch   int    `json:"-"`
	MP3Name string `json:"-"` // done 后设置

	finishedAt time.Time
}

// RunFunc 执行一次合成，返回 mp3 文件名。由 handlers 注入，避免循环依赖。
type RunFunc func(ctx context.Context, j *Job) (string, error)

type Queue struct {
	mu      sync.Mutex
	jobs    map[string]*Job
	adminQ  []*Job // admin FIFO，出队优先于 normalQ → 插队
	normalQ []*Job // 普通用户 FIFO
	notify  chan struct{}
	workers int
	run     RunFunc
}

func NewQueue(workers int, run RunFunc) *Queue {
	return &Queue{
		jobs:    make(map[string]*Job),
		notify:  make(chan struct{}, 1),
		workers: workers,
		run:     run,
	}
}

// Start 启动 worker 与清扫 goroutine。
func (q *Queue) Start() {
	for i := 0; i < q.workers; i++ {
		go q.worker()
	}
	go q.sweeper()
}

// Enqueue 入队，返回 job 与当前 position（前面等待的任务数）。
func (q *Queue) Enqueue(owner string, admin bool, text, voice string, rate, pitch int) (*Job, int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.adminQ)+len(q.normalQ) >= maxWaiting {
		return nil, 0, ErrQueueFull
	}

	j := &Job{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Status:    StatusQueued,
		CreatedAt: time.Now(),
		Owner:     owner,
		Admin:     admin,
		Text:      text,
		Voice:     voice,
		Rate:      rate,
		Pitch:     pitch,
	}
	q.jobs[j.ID] = j

	var position int
	if admin {
		position = len(q.adminQ)
		q.adminQ = append(q.adminQ, j)
	} else {
		position = len(q.adminQ) + len(q.normalQ)
		q.normalQ = append(q.normalQ, j)
	}

	select {
	case q.notify <- struct{}{}:
	default:
	}
	return j, position, nil
}

// Get 查询任务；position 为前面等待的任务数，非 queued 状态返回 -1。
func (q *Queue) Get(id string) (*Job, int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	j, ok := q.jobs[id]
	if !ok {
		return nil, -1, false
	}
	if j.Status != StatusQueued {
		return j, -1, true
	}
	return j, q.positionLocked(j), true
}

// Cancel 取消排队中的任务（running 不可打断）。
// 权限（owner/admin）由调用方校验；返回 false 表示任务不存在或不在 queued 状态。
func (q *Queue) Cancel(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	j, ok := q.jobs[id]
	if !ok || j.Status != StatusQueued {
		return false
	}
	q.removeLocked(j)
	j.Status = StatusFailed
	j.Err = "已取消"
	j.finishedAt = time.Now()
	return true
}

// positionLocked 计算 queued 任务前面还有多少等待任务。调用方须持锁。
func (q *Queue) positionLocked(j *Job) int {
	if j.Admin {
		for i, x := range q.adminQ {
			if x == j {
				return i
			}
		}
		return 0
	}
	for i, x := range q.normalQ {
		if x == j {
			return len(q.adminQ) + i
		}
	}
	return len(q.adminQ)
}

// removeLocked 把任务从等待切片中摘除。调用方须持锁。
func (q *Queue) removeLocked(j *Job) {
	queue := &q.normalQ
	if j.Admin {
		queue = &q.adminQ
	}
	for i, x := range *queue {
		if x == j {
			*queue = append((*queue)[:i], (*queue)[i+1:]...)
			return
		}
	}
}

// dequeue 取下一个待执行任务：adminQ 优先。调用方须持锁。
func (q *Queue) dequeueLocked() *Job {
	var j *Job
	if len(q.adminQ) > 0 {
		j = q.adminQ[0]
		q.adminQ = q.adminQ[1:]
	} else if len(q.normalQ) > 0 {
		j = q.normalQ[0]
		q.normalQ = q.normalQ[1:]
	}
	if j != nil {
		j.Status = StatusRunning
	}
	return j
}

func (q *Queue) worker() {
	for {
		q.mu.Lock()
		j := q.dequeueLocked()
		q.mu.Unlock()

		if j == nil {
			<-q.notify
			continue
		}

		// 用 Background 而非 request context：提交请求的 HTTP 连接早已返回
		ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
		mp3Name, err := q.run(ctx, j)
		cancel()

		q.mu.Lock()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				j.Err = "生成超时，请缩短文本后重试"
			} else {
				j.Err = err.Error()
			}
			j.Status = StatusFailed
		} else {
			j.MP3Name = mp3Name
			j.Status = StatusDone
		}
		j.finishedAt = time.Now()
		q.mu.Unlock()
	}
}

// sweeper 定期清理：queued 超 queueTTL 的任务自动放弃；
// done/failed 超 jobTTL 的条目从注册表删除（mp3 文件由 handlers 的 cleaner 清理）。
func (q *Queue) sweeper() {
	ticker := time.NewTicker(sweepPeriod)
	for now := range ticker.C {
		q.mu.Lock()
		for id, j := range q.jobs {
			switch j.Status {
			case StatusQueued:
				if now.Sub(j.CreatedAt) > queueTTL {
					q.removeLocked(j)
					j.Status = StatusFailed
					j.Err = "排队超时，请重新提交"
					j.finishedAt = now
				}
			case StatusDone, StatusFailed:
				if now.Sub(j.finishedAt) > jobTTL {
					delete(q.jobs, id)
				}
			}
		}
		q.mu.Unlock()
	}
}
