package main

import (
	"context"
	"fmt"
	"log"

	"github.com/fido-device-onboard/go-fdo/passport"
)

// Example demonstrating the TO2-specific fallback mechanism
func main() {
	fmt.Println("🔐 TO2-Specific Fallback Mechanism Example")
	fmt.Println("==========================================")

	// Create Passport client configuration
	config := &passport.PassportConfig{
		BaseURL: "https://api.passport.example.com",
		APIKey:  "your-api-key-here",
	}

	// Create Passport client
	client := passport.NewPassportClient(config)

	// Create fallback voucher state
	fallbackState := passport.NewFallbackVoucherState(client)

	fmt.Println("\n📋 Fallback Behavior:")
	fmt.Println("   1. During TO1/DI protocols: Fallback to Passport is DISABLED")
	fmt.Println("   2. During TO2 protocol: Fallback to Passport is ENABLED")
	fmt.Println("   3. Fallback only triggers when conversion layer fails")

	// Demonstrate fallback state management
	fmt.Println("\n🔒 Testing Fallback State Management:")
	
	// Initially, fallback should be disabled
	if fallbackState.IsTO2Active() {
		fmt.Println("   ❌ ERROR: Fallback should be disabled initially")
	} else {
		fmt.Println("   ✅ Fallback is correctly disabled initially")
	}

	// Enable TO2 mode
	fallbackState.SetTO2Active(true)
	if fallbackState.IsTO2Active() {
		fmt.Println("   ✅ Fallback is now enabled for TO2 protocol")
	} else {
		fmt.Println("   ❌ ERROR: Failed to enable fallback for TO2")
	}

	// Disable TO2 mode
	fallbackState.SetTO2Active(false)
	if !fallbackState.IsTO2Active() {
		fmt.Println("   ✅ Fallback is now disabled after TO2 protocol")
	} else {
		fmt.Println("   ❌ ERROR: Failed to disable fallback after TO2")
	}

	// Create TO2 server wrapper
	to2Wrapper := passport.NewTO2ServerWrapper(fallbackState)
	fmt.Println("\n🚀 TO2 Server Wrapper Created:")
	fmt.Println("   - Automatically manages TO2 state during protocol execution")
	fmt.Println("   - Fallback only enabled during TO2 message processing")
	fmt.Println("   - All other protocols (TO1, DI) will not trigger fallback")

	// Demonstrate the complete flow
	fmt.Println("\n🔄 Complete Fallback Flow:")
	fmt.Println("   1. TO2 message arrives → TO2 state enabled")
	fmt.Println("   2. Voucher lookup attempted via conversion layer")
	fmt.Println("   3. If conversion fails AND in TO2 → Passport API called")
	fmt.Println("   4. If conversion fails AND NOT in TO2 → Error returned")
	fmt.Println("   5. TO2 message completed → TO2 state disabled")

	fmt.Println("\n✅ TO2-Specific Fallback Mechanism is Ready!")
	fmt.Println("\nKey Benefits:")
	fmt.Println("   - Fallback only happens during TO2 operations")
	fmt.Println("   - TO1 and DI protocols are unaffected")
	fmt.Println("   - Automatic state management")
	fmt.Println("   - Thread-safe implementation")
}

// Example function showing how to use the fallback mechanism in a real server
func exampleServerUsage() {
	config := &passport.PassportConfig{
		BaseURL: "https://api.passport.example.com",
		APIKey:  "your-api-key-here",
	}

	client := passport.NewPassportClient(config)
	fallbackState := passport.NewFallbackVoucherState(client)

	// Create TO2 server with fallback capability
	to2Server := passport.NewTO2ServerWrapper(fallbackState)

	// The server will automatically:
	// 1. Enable fallback when TO2 messages arrive
	// 2. Disable fallback when TO2 messages complete
	// 3. Only allow Passport API calls during TO2 operations

	_ = to2Server // Use to2Server in your FDO server implementation
}

// Example function showing how the fallback prevents unauthorized Passport calls
func exampleFallbackSecurity() {
	config := &passport.PassportConfig{
		BaseURL: "https://api.passport.example.com",
		APIKey:  "your-api-key-here",
	}

	fallbackState := passport.NewFallbackVoucherState(config)

	// Simulate voucher lookup outside of TO2 protocol
	ctx := context.Background()
	var guid [16]byte // Example GUID
	
	// This will fail with fallback disabled (not in TO2)
	_, err := fallbackState.Voucher(ctx, guid)
	if err != nil {
		fmt.Printf("✅ Security enforced: %v\n", err)
		fmt.Println("   Fallback to Passport API is blocked outside TO2 protocol")
	} else {
		fmt.Println("❌ ERROR: Fallback should be blocked outside TO2")
	}
}
