package log

import (
	"bytes"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestModuleRoutingCore_RoutesByModuleField(t *testing.T) {
	encCfg := zapcore.EncoderConfig{
		MessageKey: "message",
		LevelKey:   "level",
		TimeKey:    "ts",
	}
	enc := zapcore.NewJSONEncoder(encCfg)

	var sysBuf, bizBuf, fbBuf bytes.Buffer
	sysCore := zapcore.NewCore(enc, zapcore.AddSync(&sysBuf), zapcore.DebugLevel)
	bizCore := zapcore.NewCore(enc, zapcore.AddSync(&bizBuf), zapcore.DebugLevel)
	fbCore := zapcore.NewCore(enc, zapcore.AddSync(&fbBuf), zapcore.DebugLevel)

	core := &moduleRoutingCore{
		systemCore:   sysCore,
		businessCore: bizCore,
		fallbackCore: fbCore,
	}

	entry := zapcore.Entry{Message: "hello", Level: zapcore.InfoLevel}

	// system module -> system only
	if err := core.Write(entry, []zapcore.Field{zapcore.Field{Key: "module", Type: zapcore.StringType, String: "network"}}); err != nil {
		t.Fatalf("Write(system) error: %v", err)
	}
	if sysBuf.Len() == 0 || bizBuf.Len() != 0 {
		t.Fatalf("expected system only; sys=%d biz=%d", sysBuf.Len(), bizBuf.Len())
	}
	sysBuf.Reset()
	bizBuf.Reset()

	// business module -> business only
	if err := core.Write(entry, []zapcore.Field{zapcore.Field{Key: "module", Type: zapcore.StringType, String: "tx"}}); err != nil {
		t.Fatalf("Write(business) error: %v", err)
	}
	if bizBuf.Len() == 0 || sysBuf.Len() != 0 {
		t.Fatalf("expected business only; sys=%d biz=%d", sysBuf.Len(), bizBuf.Len())
	}
	sysBuf.Reset()
	bizBuf.Reset()

	// missing module -> both (current design)
	if err := core.Write(entry, nil); err != nil {
		t.Fatalf("Write(no module) error: %v", err)
	}
	if sysBuf.Len() == 0 || bizBuf.Len() == 0 {
		t.Fatalf("expected both when module missing; sys=%d biz=%d", sysBuf.Len(), bizBuf.Len())
	}
}


