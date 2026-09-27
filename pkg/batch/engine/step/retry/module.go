package retry

import "go.uber.org/fx"

// Module provides components related to retry policies.
var Module = fx.Options(
	fx.Provide(
		func() BackoffWaiter {
			return &RealBackoffWaiter{}
		},
	),
)
