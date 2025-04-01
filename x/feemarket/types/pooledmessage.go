package types

import (
	"sync"

	"github.com/cosmos/gogoproto/proto"
)

type MessagePool[T proto.Message] struct {
	pool *sync.Pool
}

func NewMessagePool[T proto.Message](factory func() T) *MessagePool[T] {
	return &MessagePool[T]{
		pool: &sync.Pool{
			New: func() any {
				return factory()
			},
		},
	}
}

func (p *MessagePool[T]) Get() PooledMessage[T] {
	obj := p.pool.Get().(T)
	return PooledMessage[T]{
		Value: obj,
		pool:  p,
	}
}

func (p *MessagePool[T]) put(obj T) {
	p.pool.Put(obj)
}

type PooledMessage[T proto.Message] struct {
	Value T
	pool  *MessagePool[T]
}

func (p *PooledMessage[T]) Release() {
	p.pool.put(p.Value)
}
