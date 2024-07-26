package main

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type telegramHandlerWrapper[T any] struct {
	handling *telegramHandling
}

type wrappedHandler[T any] func(ctx context.Context, e tg.Entities, u T, event string, logger Logger) error

func (helper telegramHandlerWrapper[T]) wrappedRequestOnQueue(event string, op wrappedHandler[T]) func(ctx context.Context, e tg.Entities, u T) error {
	return func(ctx context.Context, e tg.Entities, u T) error {

		logger := helper.handling.bootstrap.Logger.AddRequestID(fmt.Sprintf("tg-%s-%s", event, RandStringBytesMaskImprSrcSB(5)))

		q := helper.handling.bootstrap.Queue

		q.Enqueue(func(_ context.Context) {
			defer LogPanic(logger, "telegram_handler_wrapper")
			err := op(ctx, e, u, event, logger)
			if err != nil {
				logger.Message(gelf.LOG_ERR, "telegram_handler_wrapper", "Handler returned with error", map[string]any{
					"err": err,
				})
			}
		})

		return nil
	}
}
