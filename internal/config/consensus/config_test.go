package consensus

import "testing"

func TestNew_ParsesTargetBlockTime_FromMap(t *testing.T) {
	cfg := New(map[string]interface{}{
		"target_block_time": "30s",
	})
	got := cfg.GetOptions().TargetBlockTime
	if got.String() != "30s" {
		t.Fatalf("TargetBlockTime not parsed, got=%s want=30s", got)
	}
}

func TestNew_ParsesEmergencyDownshiftParams_FromPowMap(t *testing.T) {
	cfg := New(map[string]interface{}{
		"target_block_time": "30s",
		"pow": map[string]interface{}{
			"emergency_downshift_threshold_seconds": float64(300),
			"max_emergency_downshift_bits":          float64(9),
		},
	})
	opts := cfg.GetOptions()
	if opts.POW.EmergencyDownshiftThresholdSeconds != 300 {
		t.Fatalf("EmergencyDownshiftThresholdSeconds got=%d want=300", opts.POW.EmergencyDownshiftThresholdSeconds)
	}
	if opts.POW.MaxEmergencyDownshiftBits != 9 {
		t.Fatalf("MaxEmergencyDownshiftBits got=%d want=9", opts.POW.MaxEmergencyDownshiftBits)
	}
}
