package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/neochaotic/powerlab/backend/message-bus/common"
	"github.com/neochaotic/powerlab/backend/message-bus/model"
	"github.com/neochaotic/powerlab/backend/message-bus/pkg/ysk"
	"github.com/neochaotic/powerlab/backend/message-bus/repository"
	"github.com/neochaotic/powerlab/backend/message-bus/utils"
)

// YSKService backs the "Your Smart Knowledge" home-screen pinned-card
// list. Cards arrive over the event bus (publishers emit
// powerlab:message-bus:ysk_card_upsert / _delete events), get
// persisted to the persist DB, and the REST endpoints in
// route.api_route_ysk.go read them back for the UI.
type YSKService struct {
	repository       *repository.Repository
	ws               *EventServiceWS
	eventTypeService *EventTypeService
}

const YSKOnboardingFinishedKey = "ysk_onboarding_finished"

// NewYSKService constructs a YSKService. Call Start once after the
// EventServiceWS is running.
func NewYSKService(
	repository *repository.Repository,
	ws *EventServiceWS,
	ets *EventTypeService,
) *YSKService {
	return &YSKService{
		repository:       repository,
		ws:               ws,
		eventTypeService: ets,
	}
}

// YskCardList returns every persisted YSK card in display order.
// Returns an empty slice (not nil) on repository error so callers
// can iterate without nil-checks.
func (s *YSKService) YskCardList(ctx context.Context) ([]ysk.YSKCard, error) {
	cardList, err := (*s.repository).GetYSKCardList()
	if err != nil {
		return []ysk.YSKCard{}, err
	}
	return cardList, nil
}

// UpsertYSKCard persists yskCard, replacing any existing card with
// the same id. Short-note cards (CardTypeShortNote) are ephemeral
// toast-shaped and are silently dropped instead of stored.
func (s *YSKService) UpsertYSKCard(ctx context.Context, yskCard ysk.YSKCard) error {
	// don't store short notice cards
	if yskCard.CardType == ysk.CardTypeShortNote {
		return nil
	}
	err := (*s.repository).UpsertYSKCard(yskCard)
	return err
}

// DeleteYSKCard removes every card whose id has the given prefix.
// See repository.DatabaseRepository.DeleteYSKCard for the LIKE
// semantics.
func (s *YSKService) DeleteYSKCard(ctx context.Context, id string) error {
	return (*s.repository).DeleteYSKCard(id)
}

// Start registers the YSK event types, seeds the onboarding cards
// on first boot (when init is true), and spawns the goroutine that
// translates inbound events into upsert/delete repository calls.
// Idempotent across restarts thanks to the YSKOnboardingFinishedKey
// settings flag.
func (s *YSKService) Start(init bool) {
	// 判断数据库
	if init {
		// only run once
		settings, err := (*s.repository).GetSettings(YSKOnboardingFinishedKey)

		if settings == nil && err.Error() == "record not found" {
			s.UpsertYSKCard(context.Background(), utils.ZimaOSDataStationNotice)
			s.UpsertYSKCard(context.Background(), utils.ZimaOSFileManagementNotice)
			s.UpsertYSKCard(context.Background(), utils.ZimaOSRemoteAccessNotice)
			(*s.repository).UpsertSettings(model.Settings{
				Key:   YSKOnboardingFinishedKey,
				Value: "true",
			})
		}
	}
	// register event
	s.eventTypeService.RegisterEventType(model.EventType{
		SourceID: common.SERVICENAME,
		Name:     common.EventTypeYSKCardUpsert.Name,
	})

	s.eventTypeService.RegisterEventType(model.EventType{
		SourceID: common.SERVICENAME,
		Name:     common.EventTypeYSKCardDelete.Name,
	})

	// the event is frontend event.
	// in casaos, it register by frontend. register by who call it.
	// but in zimaos ui gen 2. the frontend lose register event type.
	// so we had to register it here.
	// but i think is not a good idea. it should register by who call it.
	s.eventTypeService.RegisterEventType(model.EventType{
		SourceID: "casaos-ui",
		Name:     "casaos-ui:app:mircoapp_communicate",
	})

	channel, err := s.ws.Subscribe(common.SERVICENAME, []string{
		common.EventTypeYSKCardUpsert.Name, common.EventTypeYSKCardDelete.Name,
	})
	if err != nil {
		_log.Error(context.Background(), "failed to subscribe to event", err)
		return
	}

	go func() {
		for {
			select {
			case event, ok := <-channel:
				if !ok {
					log.Println("channel closed")
				}
				switch event.Name {
				case common.EventTypeYSKCardUpsert.Name:
					var card ysk.YSKCard
					err := json.Unmarshal([]byte(event.Properties[common.PropertyTypeCardBody.Name]), &card)
					if err != nil {
						_log.Error(context.Background(), "failed to umarshal ysk card", err)
						continue
					}
					err = s.UpsertYSKCard(context.Background(), card)
					if err != nil {
						_log.Error(context.Background(), "failed to upsert ysk card", err)
					}
				case common.EventTypeYSKCardDelete.Name:
					err = s.DeleteYSKCard(context.Background(), event.Properties[common.PropertyTypeCardID.Name])
					if err != nil {
						_log.Error(context.Background(), "failed to delete ysk card", err)
					}
				default:
					fmt.Println(event)
				}
			}
		}
	}()
}
