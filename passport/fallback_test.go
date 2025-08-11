package passport

import (
	"fmt"
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

	// Create a mock client (this should work even with the compilation error in passport.go)
	client := &PassportClient{
		config: config,
		client: nil, // We won't actually make HTTP calls in this test
	}

	// Test that we can create a fallback voucher state
	fallbackState := NewFallbackVoucherState(client)
	if fallbackState == nil {
		t.Fatal("Failed to create fallback voucher state")
	}

	// Test that we can create an integrated server
	integratedServer := NewFallbackPassportIntegratedServer(client)
	if integratedServer == nil {
		t.Fatal("Failed to create fallback integrated server")
	}

	// Test that we can get the voucher state
	voucherState := integratedServer.GetVoucherState()
	if voucherState == nil {
		t.Fatal("Failed to get voucher state from integrated server")
	}

	fmt.Printf("✅ Successfully created fallback mechanism components:\n")
	fmt.Printf("   - FallbackVoucherState: %T\n", fallbackState)
	fmt.Printf("   - PassportIntegratedServer: %T\n", integratedServer)
	fmt.Printf("   - VoucherStateInterface: %T\n", voucherState)
}

// TestFallbackVoucherStateInterface tests that the fallback state implements the interface
func TestFallbackVoucherStateInterface(t *testing.T) {
	config := &PassportConfig{
		BaseURL: "https://test.example.com",
		APIKey:  "test-key",
		Timeout: 30,
	}

	client := &PassportClient{
		config: config,
		client: nil,
	}

	fallbackState := NewFallbackVoucherState(client)

	// Test that it implements the interface
	var _ VoucherStateInterface = fallbackState

	// Test that we can call interface methods (they may fail due to nil client, but shouldn't panic)
	// We'll just test that the interface is implemented correctly
	// The actual functionality will be tested with a proper client
	t.Log("Testing interface implementation - actual calls will fail with nil client")

	fmt.Println("✅ Fallback voucher state correctly implements VoucherStateInterface")
}
