package passport

import (
	"testing"
)

// TestFallbackVoucherStateCreation tests that we can create a fallback voucher state
func TestFallbackVoucherStateCreation(t *testing.T) {
	// Create a mock config
	config := &PassportConfig{
		BaseURL: "https://test.example.com",
		APIKey:  "test-key",
		Timeout: 30,
	}

	// Create a mock client
	client := NewPassportClient(config)

	// Test that we can create a fallback voucher state
	fallbackState := NewFallbackVoucherState(client)
	if fallbackState == nil {
		t.Fatal("Failed to create fallback voucher state")
	}

	// Test that we can create a TO2 server wrapper
	to2Wrapper := NewTO2ServerWrapper(fallbackState)
	if to2Wrapper == nil {
		t.Fatal("Failed to create TO2 server wrapper")
	}

	// Test that we can create an integrated server
	integratedServer := NewPassportIntegratedServer(client)
	if integratedServer == nil {
		t.Fatal("Failed to create integrated server")
	}

	t.Logf("✅ Successfully created fallback mechanism components:")
	t.Logf("   - FallbackVoucherState: %T", fallbackState)
	t.Logf("   - TO2ServerWrapper: %T", to2Wrapper)
	t.Logf("   - PassportIntegratedServer: %T", integratedServer)
}

// TestFallbackVoucherStateInterface tests that the fallback state implements the interface
func TestFallbackVoucherStateInterface(t *testing.T) {
	config := &PassportConfig{
		BaseURL: "https://test.example.com",
		APIKey:  "test-key",
		Timeout: 30,
	}

	client := NewPassportClient(config)
	fallbackState := NewFallbackVoucherState(client)

	// Test that it implements the interface
	var _ VoucherStateInterface = fallbackState

	t.Log("✅ Fallback voucher state correctly implements VoucherStateInterface")
}

// TestTO2StateManagement tests the TO2 state management functionality
func TestTO2StateManagement(t *testing.T) {
	config := &PassportConfig{
		BaseURL: "https://test.example.com",
		APIKey:  "test-key",
		Timeout: 30,
	}

	client := NewPassportClient(config)
	fallbackState := NewFallbackVoucherState(client)

	// Initially, fallback should be disabled
	if fallbackState.IsTO2Active() {
		t.Error("Fallback should be disabled initially")
	}

	// Enable TO2 mode
	fallbackState.SetTO2Active(true)
	if !fallbackState.IsTO2Active() {
		t.Error("Failed to enable fallback for TO2")
	}

	// Disable TO2 mode
	fallbackState.SetTO2Active(false)
	if fallbackState.IsTO2Active() {
		t.Error("Failed to disable fallback after TO2")
	}

	t.Log("✅ TO2 state management working correctly")
}
