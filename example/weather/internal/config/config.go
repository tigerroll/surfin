package config

type WeatherReaderConfig struct {
	APIEndpoint string
	APIKey      string
}

// WeatherProcessorConfig holds settings specific to WeatherProcessor.
// It is currently empty but may be extended in the future.
type WeatherProcessorConfig struct{}

// WeatherWriterConfig holds settings specific to WeatherWriter.
// It is currently empty but may be extended in the future.
type WeatherWriterConfig struct{}
