package route

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/message-bus/codegen"
	"github.com/neochaotic/powerlab/backend/message-bus/pkg/ysk"
	"github.com/samber/lo"
)

// DeleteYskCard removes every YSK card whose id has the given
// prefix.
//
// Route: DELETE /v2/message_bus/ysk/cards/{id}
func (r *APIRoute) DeleteYskCard(ctx echo.Context, id string) error {
	err := r.services.YSKService.DeleteYSKCard(ctx.Request().Context(), id)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, codegen.ResponseInternalServerError{
			Message: lo.ToPtr(err.Error()),
		})
	}
	return ctx.JSON(http.StatusOK, codegen.ResponseOK{
		Message: lo.ToPtr("success"),
	})
}

// GetYskCard returns every persisted YSK pinned card in display
// order. Each row is converted to the codegen response shape via
// ysk.ToCodegenYSKCard.
//
// Route: GET /v2/message_bus/ysk/cards
func (r *APIRoute) GetYskCard(ctx echo.Context) error {
	cardList, err := r.services.YSKService.YskCardList(ctx.Request().Context())
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, codegen.ResponseInternalServerError{
			Message: lo.ToPtr(err.Error()),
		})
	}

	return ctx.JSON(http.StatusOK, codegen.ResponseGetYSKCardListOK{
		Data: lo.ToPtr(lo.Map(cardList, func(yskCard ysk.YSKCard, _ int) codegen.YSKCard {
			card, err := ysk.ToCodegenYSKCard(yskCard)
			if err != nil {
				return codegen.YSKCard{}
			}
			return card
		})),
	})
}
