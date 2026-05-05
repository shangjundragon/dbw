package dbw

import (
	"context"
	"database/sql"
	"reflect"
	"sync"
)

type HookPoint int

const (
	HookBeforeInsert HookPoint = iota
	HookAfterInsert
	HookBeforeUpdate
	HookBeforeUpdateMap
	HookAfterUpdate
	HookBeforeDelete
	HookAfterDelete
	HookAfterQuery
)

type EntityHook func(ctx context.Context, point HookPoint, entity any) error

var globalEntityHooks []EntityHook

func RegisterEntityHook(hook EntityHook) {
	globalEntityHooks = append(globalEntityHooks, hook)
}

func (q *DbWrapper[T]) callEntityHook(point HookPoint, entity any) error {
	for _, hook := range globalEntityHooks {
		if err := hook(q.ctx, point, entity); err != nil {
			return err
		}
	}
	return nil
}

type Hooks[T any] struct {
	BeforeInsert    func(q *DbWrapper[T], data *T) error
	AfterInsert     func(q *DbWrapper[T], data *T, result sql.Result) error
	BeforeUpdate    func(q *DbWrapper[T], data *T) error
	BeforeUpdateMap func(q *DbWrapper[T], values map[string]any) error
	AfterUpdate     func(q *DbWrapper[T], result sql.Result) error
	BeforeDelete    func(q *DbWrapper[T]) error
	AfterDelete     func(q *DbWrapper[T], result sql.Result) error
	AfterQuery      func(q *DbWrapper[T], data *T) error
}

var globalHooks sync.Map

func typeKey[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func RegisterHooks[T any](fn func(h *Hooks[T])) {
	key := typeKey[T]()
	raw, _ := globalHooks.LoadOrStore(key, &Hooks[T]{})
	fn(raw.(*Hooks[T]))
}

func getGlobalHooks[T any]() *Hooks[T] {
	key := typeKey[T]()
	if raw, ok := globalHooks.Load(key); ok {
		return raw.(*Hooks[T])
	}
	return nil
}

func WithHooks[T any](fn func(h *Hooks[T])) Options {
	h := &Hooks[T]{}
	fn(h)
	return func(opts *DbWrapper[any]) {
		opts.hooks = h
	}
}

func (q *DbWrapper[T]) callBeforeInsert(data *T) error {
	if gh := getGlobalHooks[T](); gh != nil && gh.BeforeInsert != nil {
		if err := gh.BeforeInsert(q, data); err != nil {
			return err
		}
	}
	if q.hooks != nil {
		if h := q.hooks.(*Hooks[T]); h.BeforeInsert != nil {
			if err := h.BeforeInsert(q, data); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *DbWrapper[T]) callAfterInsert(data *T, result sql.Result) error {
	if gh := getGlobalHooks[T](); gh != nil && gh.AfterInsert != nil {
		if err := gh.AfterInsert(q, data, result); err != nil {
			return err
		}
	}
	if q.hooks != nil {
		if h := q.hooks.(*Hooks[T]); h.AfterInsert != nil {
			if err := h.AfterInsert(q, data, result); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *DbWrapper[T]) callBeforeUpdate(data *T) error {
	if gh := getGlobalHooks[T](); gh != nil && gh.BeforeUpdate != nil {
		if err := gh.BeforeUpdate(q, data); err != nil {
			return err
		}
	}
	if q.hooks != nil {
		if h := q.hooks.(*Hooks[T]); h.BeforeUpdate != nil {
			if err := h.BeforeUpdate(q, data); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *DbWrapper[T]) callBeforeUpdateMap(values map[string]any) error {
	if gh := getGlobalHooks[T](); gh != nil && gh.BeforeUpdateMap != nil {
		if err := gh.BeforeUpdateMap(q, values); err != nil {
			return err
		}
	}
	if q.hooks != nil {
		if h := q.hooks.(*Hooks[T]); h.BeforeUpdateMap != nil {
			if err := h.BeforeUpdateMap(q, values); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *DbWrapper[T]) callAfterUpdate(result sql.Result) error {
	if gh := getGlobalHooks[T](); gh != nil && gh.AfterUpdate != nil {
		if err := gh.AfterUpdate(q, result); err != nil {
			return err
		}
	}
	if q.hooks != nil {
		if h := q.hooks.(*Hooks[T]); h.AfterUpdate != nil {
			if err := h.AfterUpdate(q, result); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *DbWrapper[T]) callBeforeDelete() error {
	if gh := getGlobalHooks[T](); gh != nil && gh.BeforeDelete != nil {
		if err := gh.BeforeDelete(q); err != nil {
			return err
		}
	}
	if q.hooks != nil {
		if h := q.hooks.(*Hooks[T]); h.BeforeDelete != nil {
			if err := h.BeforeDelete(q); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *DbWrapper[T]) callAfterDelete(result sql.Result) error {
	if gh := getGlobalHooks[T](); gh != nil && gh.AfterDelete != nil {
		if err := gh.AfterDelete(q, result); err != nil {
			return err
		}
	}
	if q.hooks != nil {
		if h := q.hooks.(*Hooks[T]); h.AfterDelete != nil {
			if err := h.AfterDelete(q, result); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *DbWrapper[T]) callAfterQuery(data *T) error {
	if gh := getGlobalHooks[T](); gh != nil && gh.AfterQuery != nil {
		if err := gh.AfterQuery(q, data); err != nil {
			return err
		}
	}
	if q.hooks != nil {
		if h := q.hooks.(*Hooks[T]); h.AfterQuery != nil {
			if err := h.AfterQuery(q, data); err != nil {
				return err
			}
		}
	}
	return nil
}
