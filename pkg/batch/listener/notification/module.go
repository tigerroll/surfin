package notification

import (
	coreport "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/core/ports"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	"go.uber.org/fx"
)

// NewNotificationJobListenerBuilder creates a ComponentBuilder for NotificationJobListener.
func NewNotificationJobListenerBuilder(notifier ports.Notifier) jsl.NotificationListenerBuilder {
	return func(
		_ *config.Config,
		_ map[string]interface{},
	) (coreport.JobExecutionListener, error) {
		listener := NewNotificationListenerImpl(notifier)
		return &NotificationListenerAdapter{NotificationListener: listener}, nil
	}
}

// NotificationListenerParams defines the dependencies that RegisterNotificationListener receives from Fx.
type NotificationListenerParams struct {
	fx.In
	JobFactory *support.JobFactory
	Builder    jsl.NotificationListenerBuilder `name:"notificationJobListener"`
}

// RegisterNotificationListener registers the notification listener builder with the JobFactory.
func RegisterNotificationListener(p NotificationListenerParams) {
	// Defines the name referenced in JSL. Here, it is "notificationJobListener".
	p.JobFactory.RegisterNotificationListenerBuilder("notificationJobListener", p.Builder)
	logger.Debugf("Notification listener registered with JobFactory.")
}

// Module provides notification-related components.
var Module = fx.Options(
	// 1. Provides a concrete implementation of Notifier.
	fx.Provide(fx.Annotate(
		NewDummyNotifier,
		fx.As(new(ports.Notifier)),
	)),

	// 2. Provides listener builders.
	fx.Provide(fx.Annotate(NewNotificationJobListenerBuilder, fx.ResultTags(`name:"notificationJobListener"`))),

	// 3. Registers listeners with JobFactory.
	fx.Invoke(RegisterNotificationListener),
)
