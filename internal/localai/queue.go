package localai

import (
	"context"
	"sync"

	"elt/internal/ai"
)

type queue struct {
	once sync.Once
	high chan queueItem
	low  chan queueItem
}

type queueItem struct {
	ctx      context.Context
	client   *ai.Client
	task     ai.Task
	messages []ai.Message
	result   chan queueResult
}

type queueResult struct {
	res ai.Result
	err error
}

func (q *queue) ensureStarted() {
	q.once.Do(func() {
		q.high = make(chan queueItem, 64)
		q.low = make(chan queueItem, 64)
		go q.run()
	})
}

func (q *queue) run() {
	for {
		select {
		case item := <-q.high:
			q.process(item)
			continue
		default:
		}
		select {
		case item := <-q.high:
			q.process(item)
		case item := <-q.low:
			q.process(item)
		}
	}
}

func (q *queue) process(item queueItem) {
	res, err := item.client.Chat(item.ctx, item.task, item.messages)
	item.result <- queueResult{res: res, err: err}
}

func (q *queue) submit(ctx context.Context, priority bool, client *ai.Client, task ai.Task, messages []ai.Message) (ai.Result, error) {
	q.ensureStarted()

	if err := ctx.Err(); err != nil {
		return ai.Result{}, err
	}

	item := queueItem{ctx: ctx, client: client, task: task, messages: messages, result: make(chan queueResult, 1)}
	ch := q.low
	if priority {
		ch = q.high
	}
	select {
	case ch <- item:
	case <-ctx.Done():
		return ai.Result{}, ctx.Err()
	}
	select {
	case r := <-item.result:
		return r.res, r.err
	case <-ctx.Done():
		return ai.Result{}, ctx.Err()
	}
}
