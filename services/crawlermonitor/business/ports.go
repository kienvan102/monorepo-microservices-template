package business

import (
	"context"

	"github.com/kienvan102/monorepo-microservices-template/services/crawlermonitor/entity"
)

type MetricData interface {
	Write(ctx context.Context, results []entity.Result) error
}