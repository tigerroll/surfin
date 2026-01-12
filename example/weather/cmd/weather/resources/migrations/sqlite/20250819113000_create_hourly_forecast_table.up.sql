CREATE TABLE IF NOT EXISTS hourly_forecast (
    time DATETIME NOT NULL,
    weather_code INTEGER NOT NULL,
    temperature_2m REAL NOT NULL,
    latitude REAL NOT NULL,
    longitude REAL NOT NULL,
    collected_at DATETIME NOT NULL,
    PRIMARY KEY (time, latitude, longitude)
);
