package service

import (
	"testing"

	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
)

func TestSearch(t *testing.T) {
	logger.LogInitConsoleOnly()

	if d, e := NewOtherService().Search("test"); e != nil || d == nil {
		t.Error("then test search error", e)
	}
}
