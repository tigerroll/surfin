CREATE TABLE IF NOT EXISTS hourly_forecast (
    time TIMESTAMP NOT NULL,
    weather_code INTEGER NOT NULL,
    temperature_2m NUMERIC(5, 2) NOT NULL,
    latitude NUMERIC(9, 6) NOT NULL,
    longitude NUMERIC(9, 6) NOT NULL,
    collected_at TIMESTAMP NOT NULL,
    PRIMARY KEY (time, latitude, longitude)
);
