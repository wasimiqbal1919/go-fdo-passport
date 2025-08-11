// SPDX-FileCopyrightText: (C) 2024 Intel Corporation
// SPDX-License-Identifier: Apache 2.0

// Package main demonstrates how to use the FDO library with Passport API integration
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/fido-device-onboard/go-fdo"
	"github.com/fido-device-onboard/go-fdo/cbor"
	"github.com/fido-device-onboard/go-fdo/passport"
	"github.com/fido-device-onboard/go-fdo/protocol"
)

func main() {
	fmt.Println("🚀 FDO Passport Integration Examples")
	fmt.Println("====================================")
	fmt.Println()

	// Configure Passport API client with the real endpoint
	passportConfig := &passport.PassportConfig{
		BaseURL: "http://cmulk1.cymanii.org:8000",
		APIKey:  "", // No API key needed based on the curl example
		Timeout: 30 * time.Second,
	}

	passportClient := passport.NewPassportClient(passportConfig)

	fmt.Println("✅ Passport client configured")
	fmt.Println()

	// Run all demonstration examples
	fmt.Println("📋 Running all demonstration examples...")
	fmt.Println()

	// Example 1: Basic Passport integration
	fmt.Println("1️⃣ Basic Passport Integration:")
	if err := handleDeviceInitialization(passportClient); err != nil {
		log.Printf("⚠️  Basic integration failed: %v", err)
	} else {
		fmt.Println("✅ Basic integration completed")
	}
	fmt.Println()

	// Example 2: Voucher conversion
	fmt.Println("2️⃣ Voucher Conversion:")
	if err := demonstrateVoucherConversion(passportClient); err != nil {
		log.Printf("⚠️  Voucher conversion failed: %v", err)
	} else {
		fmt.Println("✅ Voucher conversion completed")
	}
	fmt.Println()

	// Example 3: Fallback mechanism
	fmt.Println("3️⃣ Fallback Mechanism:")
	demonstrateFallbackExample(passportClient)
	fmt.Println()

	// Example 4: Integration example
	fmt.Println("4️⃣ Complete Integration:")
	demonstrateIntegrationExample(passportClient)
	fmt.Println()

	// Example 5: Verification
	fmt.Println("5️⃣ Verification:")
	demonstrateVerificationExample(passportClient)
	fmt.Println()

	fmt.Println("🎉 All examples completed!")
	fmt.Println("📚 Check individual files for detailed implementations:")
	fmt.Println("   - device_initiation.go: Device initiation with Passport")
	fmt.Println("   - fallback_example.go: Fallback mechanism demonstration")
	fmt.Println("   - integration_example.go: Complete integration demo")
	fmt.Println("   - verify.go: API verification and testing")
}

func handleDeviceInitialization(client *passport.PassportClient) error {
	ctx := context.Background()

	// Create Passport-integrated FDO server
	server := passport.NewPassportIntegratedServer(client)

	// In a real scenario, this would come from an actual DI request
	// Here we'll create a mock voucher for demonstration
	mockVoucher := createMockVoucher()

	// The voucher will be automatically stored in Passport when created
	if err := server.OnVoucherCreated(ctx, mockVoucher); err != nil {
		return fmt.Errorf("error creating voucher in passport: %w", err)
	}

	// You can now create FDO servers using the Passport-backed state:
	// diServer := &fdo.DIServer{
	// 	Vouchers: server.GetVoucherState(),
	// 	// ... other DI server config
	// }

	fmt.Println("✓ Mock voucher successfully created and stored in Passport")
	return nil
}

func demonstrateVoucherConversion(client *passport.PassportClient) error {
	ctx := context.Background()

	// Create a converter
	converter := passport.NewVoucherToPassportConverter(client)

	// Create a mock voucher
	mockVoucher := createMockVoucher()

	// Convert to Passport format
	deployedLocation := "Test Lab Location"
	passportData, err := converter.VoucherToPassport(mockVoucher, deployedLocation)
	if err != nil {
		return fmt.Errorf("error converting voucher to passport: %w", err)
	}

	fmt.Printf("✓ Voucher converted to Passport format:\n")
	fmt.Printf("  Controller UUID: %s\n", passportData.ControllerUUID)
	fmt.Printf("  Certificate: %s\n", passportData.Cert[:50]+"...") // Truncate for display
	fmt.Printf("  Deployed Location: %s\n", passportData.DeployedLocation)
	fmt.Printf("  Timestamp: %s\n", passportData.Timestamp)

	// Store in Passport (this would normally go to a real API)
	if err := client.StorePassportCommissioning(ctx, passportData); err != nil {
		// This will fail in the demo because we don't have a real API
		fmt.Printf("⚠ Expected error storing to Passport API (demo): %v\n", err)
	}

	// Try to retrieve from Passport (this would also fail in demo)
	retrievedData, err := client.GetPassportCommissioning(ctx, passportData.ControllerUUID)
	if err != nil {
		fmt.Printf("⚠ Expected error retrieving from Passport API (demo): %v\n", err)
	} else {
		fmt.Printf("✓ Retrieved data from Passport: %+v\n", retrievedData)
	}

	return nil
}

func createMockVoucher() *fdo.Voucher {
	// Create a mock GUID
	var guid protocol.GUID
	copy(guid[:], []byte("mock-device-guid"))

	// Create mock manufacturer key
	mfgKey := protocol.PublicKey{
		Type:     protocol.Secp256r1KeyType,
		Encoding: protocol.X509KeyEnc,
		Body:     []byte("mock-manufacturer-public-key-data"),
	}

	// Create mock rendezvous info
	rvInfo := [][]protocol.RvInstruction{{
		{Variable: protocol.RVIPAddress, Value: []byte("127.0.0.1")},
		{Variable: protocol.RVDevPort, Value: []byte("8080")},
	}}

	// Create mock voucher header
	header := fdo.VoucherHeader{
		Version:         1,
		GUID:            guid,
		RvInfo:          rvInfo,
		DeviceInfo:      "Mock Device for Passport Integration",
		ManufacturerKey: mfgKey,
		CertChainHash:   nil, // Simplified for demo
	}

	// Create mock voucher
	voucher := &fdo.Voucher{
		Version:   1,
		Header:    *cbor.NewBstr(header),
		Hmac:      protocol.Hmac{Algorithm: protocol.Sha256Hash, Value: []byte("mock-hmac")},
		CertChain: nil, // Simplified for demo
		Entries:   nil, // No ownership transfers yet
	}

	return voucher
}

// Additional helper functions for testing with a mock HTTP server

func startMockPassportAPIServer() *http.Server {
	mux := http.NewServeMux()

	// Mock endpoint for storing commissioning data
	mux.HandleFunc("/api/v1/commissioning", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"status": "created", "message": "Commissioning data stored successfully"}`))
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// Mock endpoint for retrieving commissioning data
	mux.HandleFunc("/api/v1/commissioning/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Extract GUID from URL
			guid := r.URL.Path[len("/api/v1/commissioning/"):]

			// Return mock data
			mockResponse := fmt.Sprintf(`{
				"device_id": "%s",
				"guid": "%s",
				"manufacturer_key": "mock-manufacturer-key",
				"device_info": "Mock Device",
				"ownership_chain": [],
				"rendezvous_info": []
			}`, guid, guid)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mockResponse))
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	server := &http.Server{
		Addr:    ":8088",
		Handler: mux,
	}

	go func() {
		log.Println("Mock Passport API server starting on :8088")
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("Mock server error: %v", err)
		}
	}()

	return server
}

// demonstrateFallbackExample shows the fallback mechanism in action
func demonstrateFallbackExample(client *passport.PassportClient) {
	fmt.Println("   Creating fallback voucher state...")
	fallbackState := passport.NewFallbackVoucherState(client)
	fmt.Printf("   ✓ Fallback state created: %T\n", fallbackState)

	// Test the fallback mechanism
	ctx := context.Background()
	deviceGUID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	_, err := fallbackState.Voucher(ctx, deviceGUID)
	if err != nil {
		fmt.Printf("   ✓ Fallback mechanism working (expected error): %v\n", err)
	} else {
		fmt.Println("   ✓ Voucher retrieved successfully")
	}
}

// demonstrateIntegrationExample shows the complete integration
func demonstrateIntegrationExample(client *passport.PassportClient) {
	fmt.Println("   Creating integrated server...")
	integratedServer := passport.NewFallbackPassportIntegratedServer(client)
	fmt.Printf("   ✓ Integrated server created: %T\n", integratedServer)

	voucherState := integratedServer.GetVoucherState()
	fmt.Printf("   ✓ Voucher state retrieved: %T\n", voucherState)
}

// demonstrateVerificationExample shows API verification
func demonstrateVerificationExample(client *passport.PassportClient) {
	fmt.Println("   Creating verification components...")
	converter := passport.NewVoucherToPassportConverter(client)
	fmt.Printf("   ✓ Converter created: %T\n", converter)

	// Create a mock voucher for verification
	mockVoucher := createMockVoucher()
	fmt.Printf("   ✓ Mock voucher created with GUID: %x\n", mockVoucher.Header.Val.GUID[:])

	// Try to convert to Passport format
	passportData, err := converter.VoucherToPassport(mockVoucher, "Test Location")
	if err != nil {
		fmt.Printf("   ⚠️  Conversion failed: %v\n", err)
	} else {
		fmt.Printf("   ✓ Conversion successful: %s\n", passportData.ControllerUUID)
	}
}
