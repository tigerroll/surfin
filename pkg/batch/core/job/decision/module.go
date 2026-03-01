package decision

import (
	flowComponent "github.com/tigerroll/surfin/pkg/batch/component/flow" // New implementation path
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	"github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	"github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	"go.uber.org/fx"
)

// NewConditionalDecisionBuilder creates a builder for the generic ConditionalDecision.
func NewConditionalDecisionBuilder() jsl.ConditionalDecisionBuilder {
	return func(id string, properties map[string]interface{}, res port.ExpressionResolver) (port.Decision, error) {
		decision := flowComponent.NewConditionalDecision(id)
		// SetResolver is not needed as the Decision implementation now holds the ExpressionResolver internally.
		decision.SetProperties(properties)
		decision.SetResolver(res) // Set the ExpressionResolver
		return decision, nil
	}
}

// RegisterDecisionBuilders registers the framework's generic decision builder with the JobFactory.
func RegisterDecisionBuilders(
	jf *support.JobFactory,
	builder jsl.ConditionalDecisionBuilder,
) {
	// "conditionalDecision" is the key used in the 'ref' attribute of JSL decision elements.
	jf.RegisterDecisionBuilder("conditionalDecision", builder)
	logger.Debugf("Generic Decision Builder 'conditionalDecision' registered with JobFactory.")
}

// Module defines Fx options for generic decision-related components provided by the framework.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewConditionalDecisionBuilder,
		// ParamTags are not needed as the dependency on ExpressionResolver is handled internally by the builder.
		fx.ResultTags(`name:"conditionalDecision"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterDecisionBuilders,
		fx.ParamTags(``, `name:"conditionalDecision"`),
	)),
)
