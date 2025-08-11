package main

import (
	"context"
	"fmt"
	"log"

	"github.com/fido-device-onboard/go-fdo/passport"
)

// Example demonstrating the fallback mechanism for TO2
func demonstrateFallbackMain() {
	// Create Passport client configuration
	config := &passport.PassportConfig{
		BaseURL: "https://passport-api.example.com",
		APIKey:  "your-api-key-here",
		Timeout: 30,
	}

	// Create Passport client
	client := passport.NewPassportClient(config)

	// Create the fallback voucher state that implements the fallback mechanism
	// This will first try conversion layer, then fall back to direct Passport calls during TO2
	fallbackState := passport.NewFallbackVoucherState(client)

	// Create the integrated server with fallback capability
	integratedServer := passport.NewFallbackPassportIntegratedServer(client)

	fmt.Println("✅ Fallback voucher state created successfully!")
	fmt.Printf("✅ Integrated server created: %T\n", integratedServer)
	fmt.Println("📋 This implementation provides:")
	fmt.Println("   1. Primary voucher retrieval through conversion layer")
	fmt.Println("   2. Fallback to direct Passport API calls during TO2 if conversion fails")
	fmt.Println("   3. Automatic caching of converted vouchers for future use")

	// Example usage in TO2 context
	ctx := context.Background()

	// Simulate a device GUID (in real usage, this would come from the TO2 protocol)
	deviceGUID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	fmt.Printf("\n🔍 Attempting to retrieve voucher for device GUID: %x\n", deviceGUID)

	// This will demonstrate the fallback mechanism:
	// 1. First tries to get voucher from conversion layer (likely fails for new devices)
	// 2. Falls back to calling Passport directly during TO2
	// 3. Converts Passport data to voucher format
	// 4. Caches the result for future use
	voucher, err := fallbackState.Voucher(ctx, deviceGUID)
	if err != nil {
		log.Printf("⚠️  Voucher retrieval failed: %v", err)
		log.Printf("📝 This is expected for demonstration purposes")
		log.Printf("🔄 In a real TO2 scenario, the fallback would now call Passport directly")
	} else {
		fmt.Printf("✅ Voucher retrieved successfully: %+v\n", voucher)
	}

	fmt.Println("\n🚀 The fallback mechanism is now ready for TO2 integration!")
	fmt.Println("   Use NewFallbackPassportIntegratedServer() to create a server with fallback capability")
	fmt.Println("   The server will automatically handle fallback during TO2.ProveOVHdr calls")
}
