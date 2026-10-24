package api

import (
	"context"
	"sync"

	"elt/internal/ai"
)

type aiQueue struct {
	once sync.Once
	high chan aiQueueItem
	low  chan aiQueueItem
}

type aiQueueItem struct {
	ctx      context.Context
	client   *ai.Client
	task     ai.Task
	messages []ai.Message
	result   chan aiQueueResult
}

type aiQueueResult struct {
	res ai.Result
	err error
}

func (q *aiQueue) ensureStarted() {
	q.once.Do(func() {

		q.high = make(chan aiQueueItem, 64)
		q.low = make(chan aiQueueItem, 64)
		go q.run()
	})
}

func (q *aiQueue) run() {
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

func (q *aiQueue) process(item aiQueueItem) {
	res, err := item.client.Chat(item.ctx, item.task, item.messages)
	item.result <- aiQueueResult{res: res, err: err}
}

func (q *aiQueue) submit(ctx context.Context, priority bool, client *ai.Client, task ai.Task, messages []ai.Message) (ai.Result, error) {
	q.ensureStarted()

	if err := ctx.Err(); err != nil {
		return ai.Result{}, err
	}

	item := aiQueueItem{ctx: ctx, client: client, task: task, messages: messages, result: make(chan aiQueueResult, 1)}
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
