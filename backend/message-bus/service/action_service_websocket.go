package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/neochaotic/powerlab/backend/message-bus/common"
	"github.com/neochaotic/powerlab/backend/message-bus/model"
)

// ActionServiceWS is the in-process pub/sub for actions — the
// request-shaped sibling of EventServiceWS. Same fan-out semantics:
// callers Trigger, subscribers receive over a buffered channel,
// 10s heartbeat, drop-on-no-listener.
type ActionServiceWS struct {
	typeService *ActionTypeService

	ctx   *context.Context
	mutex sync.Mutex
	stop  chan struct{}

	inboundChannel     chan model.Action
	subscriberChannels map[string]map[string][]chan model.Action
}

// Trigger enqueues action for fan-out. Fills in Timestamp from
// time.Now if zero. Non-blocking: silently dropped if no listener
// is draining the inbound channel.
func (s *ActionServiceWS) Trigger(action model.Action) {
	if s.inboundChannel == nil {
		_log.Error(context.Background(), "error when triggering action via websocket", ErrInboundChannelNotFound)
	}

	if action.Timestamp == 0 {
		action.Timestamp = time.Now().Unix()
	}

	// TODO - ensure properties are valid for action type

	select {
	case s.inboundChannel <- action:

	case <-(*s.ctx).Done():
		if err := (*s.ctx).Err(); err != nil {
			_log.Info(context.Background(), err.Error())
		}
		return

	default: // drop action if no one is listening
	}
}

// Subscribe returns a buffered channel that will receive every
// action whose (SourceID, Name) matches sourceID + one of names.
// Empty names means "every registered action type for sourceID".
// Returns ErrActionNameNotFound if a requested name is unknown.
func (s *ActionServiceWS) Subscribe(sourceID string, names []string) (chan model.Action, error) {
	if len(names) == 0 {
		actionTypes, err := s.typeService.GetActionTypesBySourceID(sourceID)
		if err != nil {
			return nil, err
		}

		for _, actionType := range actionTypes {
			names = append(names, actionType.Name)
		}
	}

	for _, name := range names {
		actionType, err := s.typeService.GetActionType(sourceID, name)
		if err != nil {
			return nil, err
		}

		if actionType == nil {
			return nil, ErrActionNameNotFound
		}
	}

	if s.subscriberChannels == nil {
		s.subscriberChannels = make(map[string]map[string][]chan model.Action)
	}

	if s.subscriberChannels[sourceID] == nil {
		s.subscriberChannels[sourceID] = make(map[string][]chan model.Action)
	}

	c := make(chan model.Action, 1)

	for _, name := range names {
		if s.subscriberChannels[sourceID][name] == nil {
			s.subscriberChannels[sourceID][name] = make([]chan model.Action, 0)
		}
		s.subscriberChannels[sourceID][name] = append(s.subscriberChannels[sourceID][name], c)
	}

	return c, nil
}

// Unsubscribe removes channel c from the subscriber list for
// (sourceID, name). Caller still owns c and must close it.
func (s *ActionServiceWS) Unsubscribe(sourceID string, name string, c chan model.Action) error {
	if s.subscriberChannels == nil {
		return ErrSubscriberChannelsNotFound
	}

	if s.subscriberChannels[sourceID] == nil {
		return ErrActionSourceIDNotFound
	}

	if s.subscriberChannels[sourceID][name] == nil {
		return ErrActionNameNotFound
	}

	for i, subscriber := range s.subscriberChannels[sourceID][name] {
		s.mutex.Lock()
		defer s.mutex.Unlock()

		if subscriber == c {
			if i >= len(s.subscriberChannels[sourceID][name]) {
				_log.Error(context.Background(), "the i-th subscriber is removed before we get here - concurrency issue?", nil, slog.Int("subscriber", i), slog.Int("total", len(s.subscriberChannels[sourceID][name])))
				return ErrAlreadySubscribed
			}
			s.subscriberChannels[sourceID][name] = append(s.subscriberChannels[sourceID][name][:i], s.subscriberChannels[sourceID][name][i+1:]...)
			return nil
		}
	}

	return nil
}

// Start runs the dispatcher loop until ctx is cancelled. Blocks —
// invoke as a goroutine. Same shape as EventServiceWS.Start.
func (s *ActionServiceWS) Start(ctx *context.Context) {
	s.ctx = ctx
	s.mutex = sync.Mutex{}

	s.inboundChannel = make(chan model.Action)
	s.subscriberChannels = make(map[string]map[string][]chan model.Action)
	s.stop = make(chan struct{})

	defer func() {
		if s.subscriberChannels != nil {
			for sourceID, source := range s.subscriberChannels {
				for actionName, subscribers := range source {
					for _, subscriber := range subscribers {
						select {
						case _, ok := <-subscriber:
							if ok {
								close(subscriber)
							}
						default:
							continue
						}
					}
					delete(s.subscriberChannels[sourceID], actionName)
				}
				delete(s.subscriberChannels, sourceID)
			}
			s.subscriberChannels = nil
		}

		close(s.inboundChannel)
		s.inboundChannel = nil

		close(s.stop)
		s.stop = nil
	}()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {

		case <-(*s.ctx).Done():
			return

		case action, ok := <-s.inboundChannel:
			if !ok {
				return
			}

			if s.subscriberChannels == nil {
				continue
			}

			if s.subscriberChannels[action.SourceID] == nil {
				continue
			}

			if s.subscriberChannels[action.SourceID][action.Name] == nil {
				continue
			}

			for _, c := range s.subscriberChannels[action.SourceID][action.Name] {
				select {
				case c <- action:
				case <-(*s.ctx).Done():
					return
				default: // drop event if no one is listening
					continue
				}
			}

		case <-ticker.C:
			if s.subscriberChannels == nil {
				continue
			}

			heartbeat := model.Action{
				SourceID:  common.MessageBusSourceID,
				Name:      common.MessageBusHeartbeatName,
				Timestamp: time.Now().Unix(),
			}

			for _, source := range s.subscriberChannels {
				for _, subscribers := range source {
					for _, subscriber := range subscribers {
						select {
						case subscriber <- heartbeat:
						case <-(*s.ctx).Done():
							return
						default: // drop event if no one is listening
							continue
						}
					}
				}
			}
		}
	}
}

// NewActionServiceWS constructs an ActionServiceWS bound to the
// given ActionTypeService for filter-validation. Channels allocate
// at Start.
func NewActionServiceWS(actionTypeService *ActionTypeService) *ActionServiceWS {
	return &ActionServiceWS{
		typeService: actionTypeService,
	}
}
