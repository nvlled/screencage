package main

import "sync/atomic"

type Task[T any] struct {
	Result T
	Err    error
	done   atomic.Bool
}

func (task *Task[T]) Finish()      { task.done.Store(true) }
func (task *Task[T]) IsDone() bool { return task.done.Load() }
