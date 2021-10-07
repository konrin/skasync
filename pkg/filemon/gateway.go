package filemon

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Gateway struct {
	debounce int

	timer       *time.Timer
	mu          sync.Mutex
	subscribers []func(map[string]ChangeList)
	buffer      map[string]ChangeList
}

func NewGateway(debounce int) *Gateway {
	return &Gateway{
		debounce:    debounce,
		buffer:      make(map[string]ChangeList),
		subscribers: make([]func(map[string]ChangeList), 0),
	}
}

func (g *Gateway) Start(ctx context.Context) error {
	g.timer = time.NewTimer(1<<63 - 1)
	for {
		select {
		case <-g.timer.C:
			for _, cb := range g.subscribers {
				cb(g.buffer)
			}

			g.buffer = make(map[string]ChangeList)
		case <-ctx.Done():
			g.timer.Stop()
			return nil
		}
	}
}

func (g *Gateway) RegisterProvider(ctx context.Context, name string, ch chan ChangeList) {
	go func() {
		for {
			select {
			case changeList := <-ch:
				g.mu.Lock()
				buffList, ok := g.buffer[name]
				if ok {
					changeList = changeList.Union(buffList)
				}
				g.buffer[name] = changeList
				g.mu.Unlock()

				g.timer.Reset(time.Millisecond * time.Duration(g.debounce))
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (g *Gateway) Subscribe(cb func(map[string]ChangeList)) {
	g.mu.Lock()
	g.subscribers = append(g.subscribers, cb)
	g.mu.Unlock()
}

func GatewayResultToChangeList(r map[string]ChangeList) ChangeList {
	list := NewChangeList()

	for _, l := range r {
		list = list.Union(l)
	}

	return list
}

func ToStringGatewayResult(r map[string]ChangeList) string {
	result := ""

	for providerName, l := range r {
		result += fmt.Sprintf("provider: %s\n", providerName)
		result += l.String("\t")
	}

	return result
}
