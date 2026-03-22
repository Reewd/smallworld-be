package realtime

import "context"

type NoopHub struct{}

func (NoopHub) PublishToUser(context.Context, string, string, any) error { return nil }
