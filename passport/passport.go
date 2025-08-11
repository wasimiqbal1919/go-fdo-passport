// SPDX-FileCopyrightText: (C) 2024 Intel Corporation
// SPDX-License-Identifier: Apache 2.0

// Package passport implements integration between FDO vouchers and Passport API.
// This package provides a converter that translates between FDO ownership vouchers
// and Passport commissioning data, allowing the FDO library to use Passport
// instead of traditional ownership vouchers for device management.
package passport

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fido-device-onboard/go-fdo"
	"github.com/fido-device-onboard/go-fdo/cbor"
	"github.com/fido-device-onboard/go-fdo/cose"
	"github.com/fido-device-onboard/go-fdo/protocol"
)

// PassportConfig holds configuration for Passport API integration
type PassportConfig struct {
	// BaseURL is the base URL for the Passport API
	BaseURL string

	// APIKey for authentication with Passport API
	APIKey string

	// Timeout for HTTP requests
	Timeout time.Duration

	// Client allows custom HTTP client configuration
	Client *http.Client
}

// PassportClient handles communication with the Passport API
type PassportClient struct {
	config *PassportConfig
	client *http.Client
}

// PassportCommissioningData represents the device data in Passport format
// This matches the actual Passport API specification
type PassportCommissioningData struct {
	ControllerUUID   string `json:"controller_uuid"`
	Cert             string `json:"cert"`
	DeployedLocation string `json:"deployed_location"`
	Timestamp        string `json:"timestamp"`
}

// PassportProductItemResponse represents the response from the product_item query API
// This matches the actual response structure shown in the API documentation
type PassportProductItemResponse struct {
	SchemaVersion float64 `json:"schema_version"`
	UUID          string  `json:"uuid"`
	Records       []struct {
		UUID       string `json:"uuid"`
		Signature  string `json:"signature"`
		Descriptor string `json:"descriptor"`
	} `json:"records"`
}

// PassportOwnerEntry represents an ownership transfer entry in Passport format
type PassportOwnerEntry struct {
	OwnerID      string                 `json:"owner_id"`
	PublicKey    string                 `json:"public_key"`
	Signature    string                 `json:"signature"`
	Timestamp    time.Time              `json:"timestamp"`
	PreviousHash string                 `json:"previous_hash"`
	HeaderHash   string                 `json:"header_hash"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
}

// PassportRvInfo represents rendezvous information in Passport format
type PassportRvInfo struct {
	Variable string `json:"variable"`
	Value    string `json:"value"`
}

// VoucherToPassportConverter handles conversion between FDO vouchers and Passport data
type VoucherToPassportConverter struct {
	client *PassportClient
}

// NewPassportClient creates a new Passport API client
func NewPassportClient(config *PassportConfig) *PassportClient {
	client := config.Client
	if client == nil {
		timeout := config.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		client = &http.Client{
			Timeout: timeout,
		}
	}

	return &PassportClient{
		config: config,
		client: client,
	}
}

// Config returns the PassportClient configuration
func (c *PassportClient) Config() *PassportConfig {
	return c.config
}

// NewVoucherToPassportConverter creates a new converter instance
func NewVoucherToPassportConverter(client *PassportClient) *VoucherToPassportConverter {
	return &VoucherToPassportConverter{
		client: client,
	}
}

// VoucherToPassport converts an FDO ownership voucher to Passport format
func (c *VoucherToPassportConverter) VoucherToPassport(voucher *fdo.Voucher, deployedLocation string) (*PassportCommissioningData, error) {
	if voucher == nil {
		return nil, fmt.Errorf("voucher cannot be nil")
	}

	// Convert GUID to string (this becomes the controller_uuid)
	controllerUUID := fmt.Sprintf("%x", voucher.Header.Val.GUID[:])

	// Extract certificate data
	var certData string
	if voucher.CertChain != nil && len(*voucher.CertChain) > 0 {
		// Use the first certificate in the chain as the main cert
		certData = fmt.Sprintf("%x", (*voucher.CertChain)[0].Raw)
	} else {
		// If no certificate chain, use manufacturer key as certificate placeholder
		certData = fmt.Sprintf("type_%d_enc_%d_body_%x",
			voucher.Header.Val.ManufacturerKey.Type,
			voucher.Header.Val.ManufacturerKey.Encoding,
			voucher.Header.Val.ManufacturerKey.Body)
	}

	// Generate timestamp (nanoseconds since Unix epoch)
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano())

	// Use deployed location if provided, otherwise try to extract from device info or use default
	if deployedLocation == "" {
		deployedLocation = voucher.Header.Val.DeviceInfo
		if deployedLocation == "" {
			deployedLocation = "Unknown Location"
		}
	}

	return &PassportCommissioningData{
		ControllerUUID:   controllerUUID,
		Cert:             certData,
		DeployedLocation: deployedLocation,
		Timestamp:        timestamp,
	}, nil
}

// PassportToVoucher converts Passport data back to an FDO ownership voucher
// This provides the reverse mapping pathway from Passport API response to FDO Voucher
func (c *VoucherToPassportConverter) PassportToVoucher(passportData *PassportCommissioningData) (*fdo.Voucher, error) {
	if passportData == nil {
		return nil, fmt.Errorf("passport data cannot be nil")
	}

	// Parse controller UUID back to GUID
	guid, err := parseGUIDFromPassportUUID(passportData.ControllerUUID)
	if err != nil {
		return nil, fmt.Errorf("error parsing controller UUID: %w", err)
	}

	// Parse certificate data back to manufacturer key
	mfgKey, certChain, err := parseCertificateData(passportData.Cert)
	if err != nil {
		return nil, fmt.Errorf("error parsing certificate data: %w", err)
	}

	// Create default RV info from passport data (this could be enhanced)
	rvInfo := createDefaultRvInfo(passportData)

	// Parse timestamp (stored for potential future use)
	_, err = parseTimestamp(passportData.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("error parsing timestamp: %w", err)
	}

	// Compute certificate chain hash if certificate chain exists
	var certChainHash *protocol.Hash
	if certChain != nil {
		certChainHash, err = computeCertChainHash(certChain)
		if err != nil {
			return nil, fmt.Errorf("error computing certificate chain hash: %w", err)
		}
	}

	// Create voucher header
	header := fdo.VoucherHeader{
		Version:         101, // FDO protocol version
		GUID:            guid,
		RvInfo:          rvInfo,
		DeviceInfo:      passportData.DeployedLocation,
		ManufacturerKey: mfgKey,
		CertChainHash:   certChainHash,
	}

	// Create HMAC placeholder (would need actual device secret in production)
	// This is a reconstruction pathway - actual HMAC would come from device
	hmacPlaceholder := protocol.Hmac{
		Algorithm: protocol.Sha256Hash,
		Value:     make([]byte, 32), // Placeholder - real HMAC computed by device
	}

	// Create voucher with reconstructed data
	voucher := &fdo.Voucher{
		Version:   101,
		Header:    *cbor.NewBstr(header),
		Hmac:      hmacPlaceholder,
		CertChain: certChain,
		Entries:   []cose.Sign1Tag[fdo.VoucherEntryPayload, []byte]{}, // Empty entries for basic voucher
	}

	return voucher, nil
}

// CreateVoucherFromProductItemResponse creates a voucher from the Passport product item response
// This is used during fallback when the conversion layer fails
func (c *VoucherToPassportConverter) CreateVoucherFromProductItemResponse(response *PassportProductItemResponse) (*fdo.Voucher, error) {
	if response == nil {
		return nil, fmt.Errorf("product item response cannot be nil")
	}

	// Find the commissioning record (PRODUCT PASSPORT)
	var commissioningRecord *struct {
		UUID       string `json:"uuid"`
		Signature  string `json:"signature"`
		Descriptor string `json:"descriptor"`
	}

	for _, record := range response.Records {
		if record.Descriptor == "PRODUCT PASSPORT" {
			commissioningRecord = &record
			break
		}
	}

	if commissioningRecord == nil {
		return nil, fmt.Errorf("no PRODUCT PASSPORT record found in response")
	}

	// Create commissioning data from the record
	commissioningData := &PassportCommissioningData{
		ControllerUUID:   response.UUID,
		Cert:             commissioningRecord.Signature,
		DeployedLocation: "Unknown", // This might need to come from a different source
		Timestamp:        fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	// Convert to voucher using the existing method
	return c.PassportToVoucher(commissioningData)
}

// StorePassportCommissioning stores commissioning data in Passport
// Uses the actual Passport API endpoint: /create-comissioning-passport
func (c *PassportClient) StorePassportCommissioning(ctx context.Context, data *PassportCommissioningData) error {
	url := fmt.Sprintf("%s/create-comissioning-passport", c.config.BaseURL)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling passport data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Note: No Authorization header in the provided API example
	// Add if needed: req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("passport API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetPassportCommissioning retrieves commissioning data from Passport
func (c *PassportClient) GetPassportCommissioning(ctx context.Context, controllerUUID string) (*PassportCommissioningData, error) {
	url := fmt.Sprintf("%s/product_item/?uuid=%s", c.config.BaseURL, controllerUUID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Note: No Authorization header in the provided API example
	// Add if needed: req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("commissioning data not found for controller UUID: %s", controllerUUID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("passport API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse the actual response structure from the product_item API
	var response PassportProductItemResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	// Extract commissioning data from the records
	// Look for a record with descriptor "PRODUCT PASSPORT" or similar
	var commissioningData *PassportCommissioningData
	for _, record := range response.Records {
		if record.Descriptor == "PRODUCT PASSPORT" || record.Descriptor == "COMMISSIONING" {
			// Found commissioning record, extract the data
			// Note: The actual data structure may need to be adjusted based on
			// how the commissioning information is stored in the records
			commissioningData = &PassportCommissioningData{
				ControllerUUID:   response.UUID,
				Cert:             record.Signature,                         // This might need adjustment based on actual data structure
				DeployedLocation: "Unknown",                                // This might need to come from a different field
				Timestamp:        fmt.Sprintf("%d", time.Now().UnixNano()), // This might need to come from a different field
			}
			break
		}
	}

	if commissioningData == nil {
		return nil, fmt.Errorf("no commissioning data found in passport records for UUID: %s", controllerUUID)
	}

	return commissioningData, nil
}

// Helper function to encode public keys (simplified)
func encodePublicKey(pubKey protocol.PublicKey) (string, error) {
	// In a real implementation, this would properly encode the public key
	// For now, we'll just encode the raw bytes as hex
	return fmt.Sprintf("type_%d_enc_%d_body_%x", pubKey.Type, pubKey.Encoding, pubKey.Body), nil
}

// Helper function to decode public keys (simplified)
func decodePublicKey(encodedKey string) (protocol.PublicKey, error) {
	// This would be the reverse of encodePublicKey
	// For now, return an empty public key
	return protocol.PublicKey{}, fmt.Errorf("public key decoding not yet implemented")
}

// parseGUIDFromPassportUUID converts passport controller UUID back to protocol.GUID
func parseGUIDFromPassportUUID(uuidStr string) (protocol.GUID, error) {
	var guid protocol.GUID
	if len(uuidStr) != 32 { // 16 bytes * 2 hex chars
		return guid, fmt.Errorf("invalid UUID length: expected 32 hex chars, got %d", len(uuidStr))
	}

	for i := 0; i < 16; i++ {
		b := uuidStr[i*2 : (i+1)*2]
		val, err := fmt.Sscanf(b, "%02x", &guid[i])
		if err != nil || val != 1 {
			return guid, fmt.Errorf("error parsing UUID hex at position %d: %s", i, b)
		}
	}

	return guid, nil
}

// parseCertificateData extracts manufacturer key and certificate chain from cert data
func parseCertificateData(certData string) (protocol.PublicKey, *[]*cbor.X509Certificate, error) {
	// Check if this is hex-encoded certificate or encoded key format
	if len(certData) > 12 && certData[:5] == "type_" {
		// This is an encoded manufacturer key (type_X_enc_Y_body_ZZZZ format)
		mfgKey, err := parseEncodedPublicKey(certData)
		if err != nil {
			return protocol.PublicKey{}, nil, fmt.Errorf("error parsing encoded public key: %w", err)
		}
		return mfgKey, nil, nil
	}

	// Try to parse as hex-encoded X.509 certificate
	certBytes, err := decodeHexString(certData)
	if err != nil {
		return protocol.PublicKey{}, nil, fmt.Errorf("error decoding certificate hex: %w", err)
	}

	// Parse X.509 certificate
	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		// If parsing as cert fails, create a mock manufacturer key from raw data
		mockKey := protocol.PublicKey{
			Type:     protocol.Rsa2048RestrKeyType,
			Encoding: protocol.X509KeyEnc,
			Body:     certBytes,
		}
		return mockKey, nil, nil
	}

	// Extract manufacturer key from certificate - determine type based on cert
	var mfgKey *protocol.PublicKey
	var keyErr error
	
	switch pubKey := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		mfgKey, keyErr = protocol.NewPublicKey(protocol.Rsa2048RestrKeyType, pubKey, false)
	case *ecdsa.PublicKey:
		mfgKey, keyErr = protocol.NewPublicKey(protocol.Secp256r1KeyType, pubKey, false)
	default:
		// For unsupported key types, create a mock key from raw data
		mockKey := protocol.PublicKey{
			Type:     protocol.Rsa2048RestrKeyType,
			Encoding: protocol.X509KeyEnc,
			Body:     certBytes,
		}
		return mockKey, nil, nil
	}
	if keyErr != nil {
		return protocol.PublicKey{}, nil, fmt.Errorf("error creating public key from certificate: %w", keyErr)
	}

	// Create certificate chain
	certChain := []*cbor.X509Certificate{
		(*cbor.X509Certificate)(cert),
	}

	return *mfgKey, &certChain, nil
}

// parseEncodedPublicKey parses the type_X_enc_Y_body_ZZZZ format
func parseEncodedPublicKey(encodedKey string) (protocol.PublicKey, error) {
	// Parse format: type_X_enc_Y_body_ZZZZ
	var keyType, encoding int
	var bodyHex string

	n, err := fmt.Sscanf(encodedKey, "type_%d_enc_%d_body_%s", &keyType, &encoding, &bodyHex)
	if err != nil || n != 3 {
		return protocol.PublicKey{}, fmt.Errorf("invalid encoded public key format: %s", encodedKey)
	}

	// Decode body
	body, err := decodeHexString(bodyHex)
	if err != nil {
		return protocol.PublicKey{}, fmt.Errorf("error decoding public key body: %w", err)
	}

	return protocol.PublicKey{
		Type:     protocol.KeyType(keyType),
		Encoding: protocol.KeyEncoding(encoding),
		Body:     body,
	}, nil
}

// decodeHexString converts hex string to bytes
func decodeHexString(hexStr string) ([]byte, error) {
	bytes := make([]byte, len(hexStr)/2)
	for i := 0; i < len(bytes); i++ {
		val, err := fmt.Sscanf(hexStr[i*2:(i+1)*2], "%02x", &bytes[i])
		if err != nil || val != 1 {
			return nil, fmt.Errorf("invalid hex at position %d: %s", i*2, hexStr[i*2:(i+1)*2])
		}
	}
	return bytes, nil
}

// createDefaultRvInfo creates basic RV info from passport data
func createDefaultRvInfo(passportData *PassportCommissioningData) [][]protocol.RvInstruction {
	// Create basic RV instructions - in production this might be configurable
	return [][]protocol.RvInstruction{
		{
			{
				Variable: protocol.RVDevPort,
				Value:    []byte{0x1f, 0x62}, // 8042 as CBOR
			},
			{
				Variable: protocol.RVIPAddress,
				Value:    []byte{127, 0, 0, 1}, // localhost
			},
			{
				Variable: protocol.RVProtocol,
				Value:    []byte{protocol.RVProtHTTP},
			},
		},
	}
}

// parseTimestamp converts passport timestamp to time value
func parseTimestamp(timestampStr string) (int64, error) {
	timestamp, err := fmt.Sscanf(timestampStr, "%d")
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp format: %s", timestampStr)
	}
	return int64(timestamp), nil
}

// computeCertChainHash computes hash of certificate chain
func computeCertChainHash(certChain *[]*cbor.X509Certificate) (*protocol.Hash, error) {
	if certChain == nil || len(*certChain) == 0 {
		return nil, nil
	}

	// Compute SHA-256 hash of certificate chain
	hasher := protocol.Sha256Hash.HashFunc().New()
	for _, cert := range *certChain {
		hasher.Write((*x509.Certificate)(cert).Raw)
	}

	return &protocol.Hash{
		Algorithm: protocol.Sha256Hash,
		Value:     hasher.Sum(nil),
	}, nil
}
