package tasks

import (
	"fmt"

	"github.com/andig/evcc/util"
)

type Handler interface {
	Test(log *util.Logger, ip string) []interface{}
}

type HandlerRegistry map[string]func(map[string]interface{}) (Handler, error)

var Registry HandlerRegistry = make(map[string]func(map[string]interface{}) (Handler, error))

func (r HandlerRegistry) Add(name string, factory func(map[string]interface{}) (Handler, error)) {
	if _, exists := r[name]; exists {
		panic(fmt.Sprintf("cannot register duplicate charger type: %s", name))
	}
	r[name] = factory
}

func (r HandlerRegistry) Get(name string) (func(map[string]interface{}) (Handler, error), error) {
	factory, exists := r[name]
	if !exists {
		return nil, fmt.Errorf("charger type not registered: %s", name)
	}
	return factory, nil
}
