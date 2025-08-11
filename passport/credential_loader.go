// SPDX-FileCopyrightText: (C) 2024 Intel Corporation
// SPDX-License-Identifier: Apache 2.0

package passport

import (
	"context"
	"crypto"
	"fmt"
	"strconv"
	"time"

	"github.com/fido-device-onboard/go-fdo"
	"github.com/fido-device-onboard/go-fdo/protocol"
)

// PassportCredentialLoader provides device credential loading from Passport API
// This integrates GetPassportCommissioning() into the FDO device initiation flow
type PassportCredentialLoader struct {
	client *PassportClient
}

// NewPassportCredentialLoader creates a credential loader that queries Passport API
func NewPassportCredentialLoader(client *PassportClient) *PassportCredentialLoader {
	return &PassportCredentialLoader{
		client: client,
	}
}

// LoadDeviceCredential retrieves device credentials from Passport API using controller UUID
// This is the missing mechanism that wires GetPassportCommissioning() into device initiation
func (loader *PassportCredentialLoader) LoadDeviceCredential(ctx context.Context, controllerUUID string) (*fdo.DeviceCredential, error) {
	// Query Passport API for commissioning data
	passportData, err := loader.client.GetPassportCommissioning(ctx, controllerUUID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving device commissioning data from Passport: %w", err)
	}

	// Convert Passport data to FDO DeviceCredential
	credential, err := loader.passportToDeviceCredential(passportData)
	if err != nil {
		return nil, fmt.Errorf("error converting Passport data to FDO device credential: %w", err)
	}

	return credential, nil
}

// LoadDeviceCredentialByGUID loads device credentials using FDO GUID
// This provides an alternative lookup method for devices that know their GUID
func (loader *PassportCredentialLoader) LoadDeviceCredentialByGUID(ctx context.Context, guid protocol.GUID) (*fdo.DeviceCredential, error) {
	// Convert GUID to controller UUID format (hex string)
	controllerUUID := fmt.Sprintf("%x", guid[:])
	return loader.LoadDeviceCredential(ctx, controllerUUID)
}

// passportToDeviceCredential converts PassportCommissioningData to FDO DeviceCredential
func (loader *PassportCredentialLoader) passportToDeviceCredential(passportData *PassportCommissioningData) (*fdo.DeviceCredential, error) {
	if passportData == nil {
		return nil, fmt.Errorf("passport data cannot be nil")
	}

	// Parse controller UUID back to GUID
	guid, err := parseGUIDFromHex(passportData.ControllerUUID)
	if err != nil {
		return nil, fmt.Errorf("error parsing controller UUID to GUID: %w", err)
	}

	// Parse certificate data to determine public key hash
	// In a real implementation, this would properly decode and hash the certificate
	publicKeyHash, err := parsePublicKeyHashFromCert(passportData.Cert)
	if err != nil {
		return nil, fmt.Errorf("error parsing public key hash from certificate: %w", err)
	}

	// Create basic RV info - in production this would be configurable
	rvInfo := [][]protocol.RvInstruction{
		{
			{
				Variable: protocol.RVDevPort,
				Value:    []byte{0x1f, 0x62}, // 8042 as CBOR uint16
			},
			{
				Variable: protocol.RVIPAddress,
				Value:    []byte{127, 0, 0, 1}, // 127.0.0.1 as bytes
			},
			{
				Variable: protocol.RVProtocol,
				Value:    []byte{protocol.RVProtHTTP}, // HTTP protocol
			},
		},
	}

	return &fdo.DeviceCredential{
		Version:       101,
		DeviceInfo:    passportData.DeployedLocation,
		GUID:          guid,
		RvInfo:        rvInfo,
		PublicKeyHash: publicKeyHash,
	}, nil
}

// parseGUIDFromHex converts hex string back to protocol.GUID
func parseGUIDFromHex(hexStr string) (protocol.GUID, error) {
	var guid protocol.GUID
	if len(hexStr) != 32 { // 16 bytes * 2 hex chars
		return guid, fmt.Errorf("invalid GUID hex string length: expected 32, got %d", len(hexStr))
	}

	for i := 0; i < 16; i++ {
		b, err := strconv.ParseUint(hexStr[i*2:(i+1)*2], 16, 8)
		if err != nil {
			return guid, fmt.Errorf("error parsing GUID hex string: %w", err)
		}
		guid[i] = byte(b)
	}

	return guid, nil
}

// parsePublicKeyHashFromCert extracts public key hash from certificate data
// This is a simplified implementation - in production would properly parse certificates
func parsePublicKeyHashFromCert(certData string) (protocol.Hash, error) {
	// For now, create a mock hash based on the cert data
	// In a real implementation, this would:
	// 1. Decode the hex certificate data
	// 2. Parse the X.509 certificate
	// 3. Extract and hash the public key
	// 4. Return the proper hash with algorithm type
	
	return protocol.Hash{
		Algorithm: protocol.Sha256Hash,
		Value:     []byte(certData)[:32], // Mock - use first 32 chars as hash
	}, nil
}

// PassportDeviceInitiator integrates Passport credential loading with FDO device initiation
type PassportDeviceInitiator struct {
	loader    *PassportCredentialLoader
	transport fdo.Transport
}

// NewPassportDeviceInitiator creates a device initiator that uses Passport for credentials
func NewPassportDeviceInitiator(client *PassportClient, transport fdo.Transport) *PassportDeviceInitiator {
	return &PassportDeviceInitiator{
		loader:    NewPassportCredentialLoader(client),
		transport: transport,
	}
}

// InitiateTO1WithPassport runs TO1 protocol using credentials from Passport API
// This is the main entry point that wires GetPassportCommissioning() into device onboarding
func (initiator *PassportDeviceInitiator) InitiateTO1WithPassport(ctx context.Context, controllerUUID string, deviceKey DeviceKey) (*TO1Response, error) {
	// Load device credentials from Passport
	credential, err := initiator.loader.LoadDeviceCredential(ctx, controllerUUID)
	if err != nil {
		return nil, fmt.Errorf("error loading device credentials from Passport: %w", err)
	}

	// Configure TO1 options
	opts := &fdo.TO1Options{
		PSS: false, // Configure based on key type
	}

	// Run TO1 protocol with Passport-loaded credentials
	result, err := fdo.TO1(ctx, initiator.transport, *credential, deviceKey.Signer, opts)
	if err != nil {
		return nil, fmt.Errorf("error during TO1 protocol execution: %w", err)
	}

	return &TO1Response{
		RedirectBlob: result,
		Credential:   *credential,
	}, nil
}

// InitiateTO2WithPassport runs TO2 protocol using credentials from Passport API
func (initiator *PassportDeviceInitiator) InitiateTO2WithPassport(ctx context.Context, controllerUUID string, deviceKey DeviceKey, to1Response *TO1Response) error {
	// Load device credentials from Passport (may have been updated)
	credential, err := initiator.loader.LoadDeviceCredential(ctx, controllerUUID)
	if err != nil {
		return fmt.Errorf("error loading device credentials from Passport for TO2: %w", err)
	}

	// Note: TO2 requires proper HMAC and devmod configuration
	// In a production implementation, you would:
	// 1. Configure proper TO2Config with credential, key, HMAC, and devmod
	// 2. Call fdo.TO2(ctx, transport, to1Response.RedirectBlob, config)
	// 3. Handle the returned updated device credential
	
	// This is a demonstration showing how the integration would work
	// The key point is that credentials are loaded from Passport via LoadDeviceCredential()
	_ = credential // Use credential to avoid unused variable warning
	_ = deviceKey  // Use deviceKey to avoid unused variable warning
	return fmt.Errorf("TO2 execution requires proper HMAC and devmod configuration - this is a demonstration of the Passport integration")
}

// DeviceKey wraps a crypto.Signer with additional FDO-specific functionality
type DeviceKey struct {
	Signer crypto.Signer
}

// TO1Response contains the result of TO1 protocol execution
type TO1Response struct {
	RedirectBlob interface{} // Should be *cose.Sign1[protocol.To1d, []byte]
	Credential   fdo.DeviceCredential
}

// Helper functions for production implementation

// ValidatePassportCredential validates that Passport data is suitable for FDO
func ValidatePassportCredential(passportData *PassportCommissioningData) error {
	if passportData == nil {
		return fmt.Errorf("passport data cannot be nil")
	}
	if passportData.ControllerUUID == "" {
		return fmt.Errorf("controller UUID cannot be empty")
	}
	if passportData.Cert == "" {
		return fmt.Errorf("certificate data cannot be empty")
	}
	if passportData.Timestamp == "" {
		return fmt.Errorf("timestamp cannot be empty")
	}
	
	// Validate timestamp format
	if _, err := strconv.ParseInt(passportData.Timestamp, 10, 64); err != nil {
		return fmt.Errorf("invalid timestamp format: %w", err)
	}
	
	return nil
}

// ConfigureRvInfoFromPassport allows customization of RV info based on Passport data
func ConfigureRvInfoFromPassport(passportData *PassportCommissioningData, defaultHost string, defaultPort string) [][]protocol.RvInstruction {
	host := defaultHost
	port := defaultPort
	
	// In a production implementation, you might extract host/port from deployed_location
	// or have additional fields in the Passport data
	
	return [][]protocol.RvInstruction{
		{
			{
				Variable: protocol.RVDevPort,
				Value:    []byte(port), // Should be CBOR-encoded
			},
			{
				Variable: protocol.RVIPAddress,
				Value:    []byte(host), // Should be properly encoded IP
			},
			{
				Variable: protocol.RVProtocol,
				Value:    []byte{protocol.RVProtHTTP}, // HTTP protocol constant
			},
		},
	}
}

// PassportCredentialCache provides caching for frequently accessed credentials
type PassportCredentialCache struct {
	loader *PassportCredentialLoader
	cache  map[string]*cachedCredential
}

type cachedCredential struct {
	credential *fdo.DeviceCredential
	timestamp  time.Time
	ttl        time.Duration
}

// NewPassportCredentialCache creates a credential cache with TTL
func NewPassportCredentialCache(loader *PassportCredentialLoader, ttl time.Duration) *PassportCredentialCache {
	return &PassportCredentialCache{
		loader: loader,
		cache:  make(map[string]*cachedCredential),
	}
}

// GetCredential retrieves credential with caching
func (cache *PassportCredentialCache) GetCredential(ctx context.Context, controllerUUID string) (*fdo.DeviceCredential, error) {
	// Check cache first
	if cached, exists := cache.cache[controllerUUID]; exists {
		if time.Since(cached.timestamp) < cached.ttl {
			return cached.credential, nil
		}
		// Cache expired
		delete(cache.cache, controllerUUID)
	}
	
	// Load from Passport
	credential, err := cache.loader.LoadDeviceCredential(ctx, controllerUUID)
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	cache.cache[controllerUUID] = &cachedCredential{
		credential: credential,
		timestamp:  time.Now(),
		ttl:        time.Hour, // Default 1 hour TTL
	}
	
	return credential, nil
}
