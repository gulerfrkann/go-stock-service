package config

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

func InitLogger() *zap.Logger {
	zapConfig := zap.NewProductionConfig()
	zapConfig.EncoderConfig.TimeKey = "timestamp"
	zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var err error
	Logger, err = zapConfig.Build()
	if err != nil {
		panic("Logger başlatılamadı: " + err.Error())
	}
	return Logger
}