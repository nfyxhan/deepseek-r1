package utils

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

type List struct {
	data *list.List
	mut  *sync.RWMutex
}

func NewList() *List {
	return &List{
		data: list.New(),
		mut:  &sync.RWMutex{},
	}
}

func (l *List) IsFront(e *list.Element) bool {
	l.mut.RLock()
	defer l.mut.RUnlock()
	return l.data.Front() == e
}

func (l *List) MoveToFront(e *list.Element) {
	l.mut.Lock()
	defer l.mut.Unlock()
	l.data.MoveToFront(e)
}

func (l *List) Push(v interface{}) *list.Element {
	l.mut.Lock()
	defer l.mut.Unlock()
	e := l.data.PushBack(v)
	return e
}

func (l *List) Remove(e *list.Element) {
	l.mut.Lock()
	defer l.mut.Unlock()
	l.data.Remove(e)
}

func (l *List) Len() int {
	l.mut.RLock()
	defer l.mut.RUnlock()
	return l.data.Len()
}

func (l *List) Back() *list.Element {
	l.mut.RLock()
	defer l.mut.RUnlock()
	if e := l.data.Back(); e != nil {
		return e
	}
	return nil
}

type Map struct {
	data map[string]*list.Element
	mut  *sync.RWMutex
}

func NewMap() *Map {
	return &Map{
		data: make(map[string]*list.Element),
		mut:  &sync.RWMutex{},
	}
}

func (m *Map) Get(key string) *list.Element {
	m.mut.RLock()
	defer m.mut.RUnlock()
	if e, ok := m.data[key]; ok {
		return e
	}
	return nil
}

func (m *Map) Set(key string, v *list.Element) {
	m.mut.Lock()
	defer m.mut.Unlock()
	m.data[key] = v
}

func (m *Map) Add(key string, v *list.Element) error {
	m.mut.Lock()
	defer m.mut.Unlock()
	if _, ok := m.data[key]; ok {
		return fmt.Errorf("%s exist", key)
	}
	m.data[key] = v
	return nil
}

func (m *Map) Remove(key string) {
	m.mut.Lock()
	defer m.mut.Unlock()
	delete(m.data, key)
}

type Element struct {
	Id   string
	Data interface{}
}

type ExpireFunc func(e *Element) bool

type LRUCache struct {
	cache      *Map
	list       *List
	mut        *sync.Mutex
	cacheSize  int
	frontChan  chan *list.Element
	stopChan   chan struct{}
	expireFunc ExpireFunc
}

func (c *LRUCache) Init(expireFunc ExpireFunc, cacheSize int) {
	c.cache = NewMap()
	c.list = NewList()
	c.mut = &sync.Mutex{}
	c.cacheSize = cacheSize
	c.frontChan = make(chan *list.Element, cacheSize)
	c.stopChan = make(chan struct{})
	c.expireFunc = expireFunc
	go c.run()
}

func (c *LRUCache) Get(key string) *Element {
	var e *list.Element
	defer func() {
		if e != nil {
			c.frontChan <- e
		}
	}()
	if e = c.cache.Get(key); e != nil {
		return e.Value.(*Element)
	}
	return nil
}

func (c *LRUCache) Add(key string, v interface{}) error {
	var e *list.Element
	if e = c.cache.Get(key); e != nil {
		return fmt.Errorf("%s exist", key)
	}
	var err error
	defer func() {
		if err == nil && e != nil {
			c.frontChan <- e
		}
	}()
	c.mut.Lock()
	defer c.mut.Unlock()
	e = c.list.Push(&Element{Data: v, Id: key})
	err = c.cache.Add(key, e)
	return err
}

func (c *LRUCache) Remove(key string) error {
	if e := c.cache.Get(key); e != nil {
		c.mut.Lock()
		defer c.mut.Unlock()
		c.list.Remove(e)
		c.cache.Remove(key)
		return nil
	}
	return fmt.Errorf("%s not exist", key)
}

func (c *LRUCache) run() {
	free := false
	for {
		select {
		case <-c.stopChan:
			return
		case e := <-c.frontChan:
			c.list.MoveToFront(e)
		case <-time.After(time.Millisecond * 100):
			free = true
		}
		if free || c.list.Len() > c.cacheSize {
			c.removeExpireFromBack()
			free = false
		}
	}
}

func (c *LRUCache) removeExpireFromBack() {
	ee := c.list.Back()
	if ee == nil {
		return
	}
	e, ok := ee.Value.(*Element)
	if !ok {
		return
	}
	if c.expireFunc(e) {
		_ = c.Remove(e.Id)
	}
}

func (c *LRUCache) Stop() {
	close(c.stopChan)
}
