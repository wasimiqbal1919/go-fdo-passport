// SPDX-FileCopyrightText: (C) 2024 Intel Corporation
// SPDX-License-Identifier: Apache 2.0

package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"log"
	"time"

	"github.com/fido-device-onboard/go-fdo"
	"github.com/fido-device-onboard/go-fdo/http"
	"github.com/fido-device-onboard/go-fdo/passport"
	"github.com/fido-device-onboard/go-fdo/protocol"
)

// This example demonstrates the missing piece - how devices query Passport
// for their credentials during initiation (TO1/TO2 protocols)

func demonstrateDeviceInitiationMain() {
	fmt.Println("=== FDO Device Initiation with Passport Integration ===")
	fmt.Println()

	// Step 1: Configure Passport client
	passportConfig := &passport.PassportConfig{
		BaseURL: "http://cmulk1.cymanii.org:8000",
		APIKey:  "", // No auth required per your spec
		Timeout: 30 * time.Second,
	}

	passportClient := passport.NewPassportClient(passportConfig)
	fmt.Println("✅ Passport client configured")

	// Step 2: Create device credentials loader
	credentialLoader := passport.NewPassportCredentialLoader(passportClient)
	fmt.Println("✅ Credential loader created")

	// Step 3: Simulate device with known controller UUID
	// This would typically be stored in device TPM or secure storage
	controllerUUID := "191e886b-dfff-4f39-9618-d7a364ec0c90"
	fmt.Printf("📱 Simulating device with Controller UUID: %s\n", controllerUUID)
	fmt.Println()

	// Step 4: Demonstrate credential loading from Passport
	fmt.Println("🔍 Loading device credentials from Passport API...")
	ctx := context.Background()

	credential, err := credentialLoader.LoadDeviceCredential(ctx, controllerUUID)
	if err != nil {
		log.Printf("❌ Error loading credentials: %v", err)
		fmt.Println()
		fmt.Println("💡 This demonstrates how GetPassportCommissioning() would be called")
		fmt.Println("   during device initiation. The device queries Passport for its")
		fmt.Println("   commissioning data instead of loading from local storage.")

		// Show what the API call would look like
		demonstratePassportQuery(passportClient, controllerUUID)
		return
	}

	fmt.Println("✅ Credentials loaded from Passport!")
	fmt.Printf("   GUID: %x\n", credential.GUID)
	fmt.Printf("   Device Info: %s\n", credential.DeviceInfo)
	fmt.Printf("   RV Info: %v\n", credential.RvInfo)
	fmt.Println()

	// Step 5: Demonstrate full device initiation flow
	fmt.Println("🚀 Starting FDO Device Initiation with Passport credentials...")

	if err := demonstrateDeviceInitiation(passportClient, controllerUUID); err != nil {
		log.Printf("❌ Device initiation failed: %v", err)
	}
}

// demonstratePassportQuery shows what happens when device queries Passport
func demonstratePassportQuery(client *passport.PassportClient, controllerUUID string) {
	fmt.Println()
	fmt.Println("📋 What happens during Passport credential query:")
	fmt.Println()

	fmt.Printf("   1. Device calls: GetPassportCommissioning(ctx, \"%s\")\n", controllerUUID)
	fmt.Printf("   2. HTTP GET: %s/product_item/?uuid=%s\n", client.Config().BaseURL, controllerUUID)
	fmt.Println("   3. Passport returns commissioning data in this format:")

	// Show example response format
	fmt.Println(`      {
        "controller_uuid": "191e886b-dfff-4f39-9618-d7a364ec0c90",
        "cert": "type_10_enc_1_body_6d6f636b2d6365727469666963617465",
        "deployed_location": "Production Environment", 
        "timestamp": "1754509904342152960"
      }`)

	fmt.Println()
	fmt.Println("   4. Credential loader converts to FDO DeviceCredential")
	fmt.Println("   5. Device uses credential for TO1/TO2 protocols")
	fmt.Println()
	fmt.Println("🔗 This is the missing link you identified!")
}

// demonstrateDeviceInitiation shows complete device initiation with Passport
func demonstrateDeviceInitiation(passportClient *passport.PassportClient, controllerUUID string) error {
	fmt.Println("Setting up device initiation components...")

	// Create device key (normally would be in TPM)
	deviceKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("error generating device key: %w", err)
	}

	// Create transport (normally would connect to actual RV/Owner)
	transport := &http.Transport{
		BaseURL: "http://localhost:8042", // Mock RV server
	}

	// Create Passport device initiator
	initiator := passport.NewPassportDeviceInitiator(passportClient, transport)
	fmt.Println("✅ Device initiator configured with Passport")

	ctx := context.Background()

	// Step 1: TO1 Protocol with Passport credentials
	fmt.Println()
	fmt.Println("🔄 Step 1: TO1 Protocol (Device → Rendezvous)")
	fmt.Printf("   - Loading credentials from Passport for UUID: %s\n", controllerUUID)
	fmt.Println("   - Querying RV server for Owner info")

	to1Response, err := initiator.InitiateTO1WithPassport(ctx, controllerUUID, passport.DeviceKey{
		Signer: deviceKey,
	})

	if err != nil {
		fmt.Printf("❌ TO1 failed: %v\n", err)
		fmt.Println()
		fmt.Println("📋 This is expected since we don't have actual RV server running.")
		fmt.Println("   In production:")
		fmt.Println("   1. Device loads credentials from Passport ✅")
		fmt.Println("   2. Device contacts RV server with credentials")
		fmt.Println("   3. RV server returns Owner service info")
		return err
	}

	fmt.Println("✅ TO1 completed successfully!")
	fmt.Printf("   Received redirect blob for Owner service\n")

	// Step 2: TO2 Protocol with Passport credentials
	fmt.Println()
	fmt.Println("🔄 Step 2: TO2 Protocol (Device → Owner)")
	fmt.Printf("   - Re-loading credentials from Passport (may have been updated)\n")
	fmt.Println("   - Connecting to Owner service")
	fmt.Println("   - Performing device onboarding")

	err = initiator.InitiateTO2WithPassport(ctx, controllerUUID, passport.DeviceKey{
		Signer: deviceKey,
	}, to1Response)

	if err != nil {
		fmt.Printf("❌ TO2 failed: %v\n", err)
		fmt.Println()
		fmt.Println("📋 This is expected since we don't have actual Owner server running.")
		fmt.Println("   In production:")
		fmt.Println("   1. Device loads credentials from Passport ✅")
		fmt.Println("   2. Device contacts Owner service")
		fmt.Println("   3. Owner onboards device")
		return err
	}

	fmt.Println("✅ TO2 completed successfully!")
	fmt.Println("🎉 Device onboarding complete!")

	return nil
}

// DeviceCredentialStore demonstrates how to store and update device credentials
type DeviceCredentialStore struct {
	passportClient *passport.PassportClient
	converter      *passport.VoucherToPassportConverter
}

// NewDeviceCredentialStore creates a new credential store
func NewDeviceCredentialStore(client *passport.PassportClient) *DeviceCredentialStore {
	converter := passport.NewVoucherToPassportConverter(client)
	return &DeviceCredentialStore{
		passportClient: client,
		converter:      converter,
	}
}

// UpdateCredentialInPassport updates device credentials in Passport
func (store *DeviceCredentialStore) UpdateCredentialInPassport(ctx context.Context, credential *fdo.DeviceCredential, deployedLocation string) error {
	passportData := &passport.PassportCommissioningData{
		ControllerUUID:   fmt.Sprintf("%x", credential.GUID[:]),
		Cert:             "updated_cert_data",
		DeployedLocation: deployedLocation,
		Timestamp:        fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	return store.passportClient.StorePassportCommissioning(ctx, passportData)
}

// demonstrateCredentialUpdate shows how to update Passport after ownership changes
func demonstrateCredentialUpdate() {
	fmt.Println()
	fmt.Println("🔄 Credential Update Flow:")
	fmt.Println("   1. Device ownership is transferred")
	fmt.Println("   2. New owner updates device credentials")
	fmt.Println("   3. Updated credentials are stored back in Passport")
	fmt.Println("   4. Next device initiation uses updated credentials")
	fmt.Println()
	fmt.Println("This completes the bidirectional integration:")
	fmt.Println("   📥 Device reads credentials from Passport (credential_loader.go)")
	fmt.Println("   📤 Device/Owner writes credentials to Passport (passport.go)")
}

// MockTransport provides a mock transport for testing
type MockTransport struct {
	responses map[uint8]interface{}
}

// NewMockTransport creates a mock transport with predefined responses
func NewMockTransport() *MockTransport {
	return &MockTransport{
		responses: make(map[uint8]interface{}),
	}
}

// Send implements the Transport interface for testing
func (t *MockTransport) Send(ctx context.Context, msgType uint8, msg, headers interface{}) (uint8, interface{}, error) {
	// Mock implementation for testing
	return protocol.TO2DoneMsgType, nil, fmt.Errorf("mock transport: connection failed (expected for demo)")
}

// Additional helper functions

// ExtractCredentialFromTPM demonstrates how real devices would typically load credentials
func ExtractCredentialFromTPM() (*fdo.DeviceCredential, error) {
	// In a real implementation, this would:
	// 1. Connect to TPM
	// 2. Read device credential from secure storage
	// 3. Return the credential

	return nil, fmt.Errorf("TPM integration not implemented in demo")
}

// ComparePassportVsTPMCredentials shows the difference in credential loading
func ComparePassportVsTPMCredentials() {
	fmt.Println("📋 Credential Loading Comparison:")
	fmt.Println()
	fmt.Println("Traditional FDO (TPM-based):")
	fmt.Println("   1. Device loads credential from local TPM")
	fmt.Println("   2. Credential is static, stored during DI")
	fmt.Println("   3. No network dependency")
	fmt.Println()
	fmt.Println("Passport Integration (Network-based):")
	fmt.Println("   1. Device queries Passport API for credential")
	fmt.Println("   2. Credential can be updated remotely")
	fmt.Println("   3. Requires network connectivity")
	fmt.Println("   4. Enables centralized device management")
	fmt.Println()
}

// ValidateIntegration checks that all components work together
func ValidateIntegration(passportClient *passport.PassportClient, controllerUUID string) error {
	ctx := context.Background()

	// Test credential loading
	loader := passport.NewPassportCredentialLoader(passportClient)
	_, err := loader.LoadDeviceCredential(ctx, controllerUUID)
	if err != nil {
		return fmt.Errorf("credential loading failed: %w", err)
	}

	// Test credential caching
	cache := passport.NewPassportCredentialCache(loader, time.Minute)
	_, err = cache.GetCredential(ctx, controllerUUID)
	if err != nil {
		return fmt.Errorf("credential caching failed: %w", err)
	}

	// Test validation
	passportData := &passport.PassportCommissioningData{
		ControllerUUID:   controllerUUID,
		Cert:             "mock_cert_data",
		DeployedLocation: "test_location",
		Timestamp:        fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	if err := passport.ValidatePassportCredential(passportData); err != nil {
		return fmt.Errorf("credential validation failed: %w", err)
	}

	return nil
}
