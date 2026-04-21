package impl

import (
	"github.com/slan/server/server-biz/internal/service"
)

type dbOpsService struct{ state *dbState }

var _ service.Ops = dbOpsService{}
