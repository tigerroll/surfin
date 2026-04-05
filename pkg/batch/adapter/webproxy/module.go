package webproxy

import (
	"encoding/json"
	"fmt"

	"go.uber.org/fx"

	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	webproxyconfig "github.com/tigerroll/surfin/pkg/batch/adapter/webproxy/config"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// NewWebProxyProviderFromConfig constructs a WebProxyProvider from the application's Config.
// It extracts WebProxyConfig settings from Config.Surfin.AdapterConfigs and passes them to the WebProxyProvider.
//
// Parameters:
//   cfg: The application's global configuration.
//
// Returns:
//   A new WebProxyProvider instance.
//   An error if configuration parsing fails.
func NewWebProxyProviderFromConfig(cfg *config.Config) (*WebProxyProvider, error) {
	// AdapterConfigs is of type interface{}, so assert it to map[string]interface{}.
	adapterConfigs, ok := cfg.Surfin.AdapterConfigs.(map[string]interface{})
	if !ok {
		// If AdapterConfigs is not present or not of the expected type, return an empty WebProxyProvider.
		// This prevents errors if WebProxy settings are not present in application.yaml.
		logger.Debugf("webproxy: AdapterConfigs is not a map[string]interface{} or not found. Returning empty provider.")
		return NewWebProxyProvider(make(map[string]webproxyconfig.WebProxyConfig)), nil
	}

	webProxyConfigs := make(map[string]webproxyconfig.WebProxyConfig)

	// Look for nested settings under the "webproxy" key.
	if webproxySection, found := adapterConfigs["webproxy"]; found {
		webproxyMap, ok := webproxySection.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("webproxy: 'webproxy' section in adapter config is not a map[string]interface{}")
		}

		for name, proxyCfg := range webproxyMap {
			cfgMap, ok := proxyCfg.(map[string]interface{})
			if !ok {
				// Skip if it's not a map[string]interface{}, as it might be another type of configuration.
				logger.Warnf("webproxy: Config for '%s' is not a map[string]interface{}. Skipping.", name)
				continue
			}

			// Log the raw config map content for debugging.
			logger.Debugf("webproxy: Raw config map for '%s': %+v", name, cfgMap)

			jsonBytes, err := json.Marshal(cfgMap)
			if err != nil {
				return nil, fmt.Errorf("webproxy: failed to marshal config for '%s': %w", name, err)
			}
			var wpCfg webproxyconfig.WebProxyConfig
			if err := json.Unmarshal(jsonBytes, &wpCfg); err != nil {
				// Skip if it cannot be unmarshaled as WebProxyConfig.
				// This can happen if other adapter configurations are mixed in.
				logger.Warnf("webproxy: Failed to unmarshal config for '%s' into WebProxyConfig: %v. Skipping.", name, err)
				continue
			}

			// Log the loaded WebProxyConfig content for debugging.
			logger.Debugf("webproxy: Loaded config for '%s': Type=%s, KeyName=%s, Key=%s, Placement=%s",
				name, wpCfg.Type, wpCfg.KeyName, wpCfg.Key, wpCfg.Placement)

			// Verify that the Type field of WebProxyConfig is a recognized Web Proxy authentication type.
			// This distinguishes it from other adapter configurations.
			if wpCfg.Type == "HMAC" || wpCfg.Type == "OAUTH2" || wpCfg.Type == "APIKEY" || wpCfg.Type == "MOCK_SERVER" || wpCfg.Type == "NONE" {
				webProxyConfigs[name] = wpCfg
			} else {
				logger.Warnf("webproxy: Unknown or unsupported WebProxyConfig type '%s' for '%s'. Skipping.", wpCfg.Type, name)
			}
		}
	}
	return NewWebProxyProvider(webProxyConfigs), nil
}

// Module is the Fx module for the webproxy package.
// It provides the WebProxyProvider and registers it to the resourceProviders group.
var Module = fx.Options(
	fx.Provide(NewWebProxyProviderFromConfig),
	fx.Provide(fx.Annotate(
		func(p *WebProxyProvider) coreAdapter.ResourceProvider {
			return p
		},
		fx.As(new(coreAdapter.ResourceProvider)),
		fx.ResultTags(`group:"resourceProviders"`),
	)),
)
