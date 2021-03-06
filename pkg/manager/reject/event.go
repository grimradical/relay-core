package reject

import (
	"context"

	"github.com/puppetlabs/relay-core/pkg/model"
)

type eventManager struct{}

func (*eventManager) Emit(ctx context.Context, data map[string]interface{}) (*model.Event, error) {
	return nil, model.ErrRejected
}

var EventManager model.EventManager = &eventManager{}
