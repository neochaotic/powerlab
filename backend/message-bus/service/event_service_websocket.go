package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/neochaotic/powerlab/backend/message-bus/common"
	"github.com/neochaotic/powerlab/backend/message-bus/model"
)

// EventServiceWS is the in-process pub/sub for events. Publishers
// call Publish; subscribers call Subscribe with a (sourceID, names)
// filter and receive a buffered channel of matching events plus a
// 10s heartbeat. Drops events when no subscriber is ready (back-
// pressure is on the producer, not the bus).
type EventServiceWS struct {
	typeService *EventTypeService

	ctx  *context.Context
	stop chan struct{}

	inboundChannel     chan model.Event
	subscriberChannels map[string]map[string][]chan model.Event
}

var mutex = &sync.Mutex{}

// Publish drops event onto the inbound channel for fan-out. Fills
// in Timestamp from time.Now if the producer left it zero. Non-
// blocking: if no listener is draining the inbound channel, the
// event is silently dropped.
func (s *EventServiceWS) Publish(event model.Event) {
	if s.inboundChannel == nil {
		_log.Error(context.Background(), "error when publishing event via websocket", ErrInboundChannelNotFound)
	}

	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}

	// TODO - ensure properties are valid for event type

	select {
	case s.inboundChannel <- event:

	case <-(*s.ctx).Done():
		if err := (*s.ctx).Err(); err != nil {
			_log.Info(context.Background(), err.Error())
		}
		return

	default: // drop event if no one is listening
	}
}

// Subscribe returns a buffered channel that will receive every
// event whose (SourceID, Name) matches sourceID + one of names.
// Empty names means "every registered event type for sourceID".
// Returns ErrEventNameNotFound if any requested name is not yet
// registered with the EventTypeService.
func (s *EventServiceWS) Subscribe(sourceID string, names []string) (chan model.Event, error) {
	if len(names) == 0 {
		eventTypes, err := s.typeService.GetEventTypesBySourceID(sourceID)
		if err != nil {
			return nil, err
		}

		for _, eventType := range eventTypes {
			names = append(names, eventType.Name)
		}
	}

	for _, name := range names {
		eventType, err := s.typeService.GetEventType(sourceID, name)
		if err != nil {
			return nil, err
		}

		if eventType == nil {
			return nil, ErrEventNameNotFound
		}
	}

	c := func() chan model.Event {
		mutex.Lock()
		defer mutex.Unlock()

		if s.subscriberChannels == nil {
			s.subscriberChannels = make(map[string]map[string][]chan model.Event)
		}

		if s.subscriberChannels[sourceID] == nil {
			s.subscriberChannels[sourceID] = make(map[string][]chan model.Event)
		}

		c := make(chan model.Event, 1)

		for _, name := range names {
			if s.subscriberChannels[sourceID][name] == nil {
				s.subscriberChannels[sourceID][name] = make([]chan model.Event, 0)
			}
			s.subscriberChannels[sourceID][name] = append(s.subscriberChannels[sourceID][name], c)
		}
		return c
	}()

	return c, nil
}

// Unsubscribe removes channel c from the subscriber list for
// (sourceID, name). Caller still owns c — Unsubscribe does NOT
// close it.
func (s *EventServiceWS) Unsubscribe(sourceID string, name string, c chan model.Event) error {
	if s.subscriberChannels == nil {
		return ErrSubscriberChannelsNotFound
	}

	if s.subscriberChannels[sourceID] == nil {
		return ErrEventSourceIDNotFound
	}

	if s.subscriberChannels[sourceID][name] == nil {
		return ErrEventNameNotFound
	}

	for i, subscriber := range s.subscriberChannels[sourceID][name] {
		mutex.Lock()
		defer mutex.Unlock()

		if subscriber == c {
			_log.Info(context.Background(), "unsubscribing from event type", slog.String("sourceID", sourceID), slog.String("name", name), slog.Int("subscriber", i))
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
// invoke as a goroutine. Initializes inbound + subscriber maps,
// fan-outs every Publish to matching subscribers, ticks a
// heartbeat to all subscribers every 10s, and tears down all
// channels on shutdown.
func (s *EventServiceWS) Start(ctx *context.Context) {
	func() {
		mutex.Lock()
		defer mutex.Unlock()

		s.ctx = ctx

		s.inboundChannel = make(chan model.Event)
		s.subscriberChannels = make(map[string]map[string][]chan model.Event)
		s.stop = make(chan struct{})
	}()

	defer func() {
		if s.subscriberChannels != nil {
			for sourceID, source := range s.subscriberChannels {
				for eventName, subscribers := range source {
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
					delete(s.subscriberChannels[sourceID], eventName)
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

		case event, ok := <-s.inboundChannel:
			if !ok {
				return
			}

			if s.subscriberChannels == nil {
				continue
			}

			if s.subscriberChannels[event.SourceID] == nil {
				continue
			}

			if s.subscriberChannels[event.SourceID][event.Name] == nil {
				continue
			}

			for _, c := range s.subscriberChannels[event.SourceID][event.Name] {
				select {
				case c <- event:
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

			heartbeat := model.Event{
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

// NewEventServiceWS constructs an EventServiceWS bound to the given
// EventTypeService for filter-validation. Channels are not allocated
// until Start is called.
func NewEventServiceWS(eventTypeService *EventTypeService) *EventServiceWS {
	return &EventServiceWS{
		typeService: eventTypeService,
	}
}
