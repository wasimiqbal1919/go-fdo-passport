// SPDX-FileCopyrightText: (C) 2024 Intel Corporation
// SPDX-License-Identifier: Apache 2.0

package passport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fido-device-onboard/go-fdo"
	"github.com/fido-device-onboard/go-fdo/protocol"
)

// PassportVoucherState implements the FDO voucher state interfaces but
// uses Passport API as the backend instead of traditional voucher storage.
// This allows the FDO library to work with Passport without changing the core logic.
type PassportVoucherState struct {
	converter *VoucherToPassportConverter
	client    *PassportClient
	
	// Cache to hold vouchers temporarily during protocol execution
	voucherCache map[string]*fdo.Voucher
}

// NewPassportVoucherState creates a new Passport-backed voucher state
func NewPassportVoucherState(client *PassportClient) *PassportVoucherState {
	converter := NewVoucherToPassportConverter(client)
	return &PassportVoucherState{
		converter:    converter,
		client:       client,
		voucherCache: make(map[string]*fdo.Voucher),
	}
}

// FallbackVoucherState implements the fallback mechanism for TO2.
// It first tries to get vouchers from the conversion layer, and if that fails,
// it falls back to calling Passport directly during TO2.
type FallbackVoucherState struct {
	primaryState *PassportVoucherState
	client       *PassportClient
	converter    *VoucherToPassportConverter
}

// NewFallbackVoucherState creates a new fallback voucher state that implements
// the fallback mechanism for TO2.
func NewFallbackVoucherState(client *PassportClient) *FallbackVoucherState {
	primaryState := NewPassportVoucherState(client)
	converter := NewVoucherToPassportConverter(client)
	return &FallbackVoucherState{
		primaryState: primaryState,
		client:       client,
		converter:    converter,
	}
}

// Implementation of ManufacturerVoucherPersistentState interface

// NewVoucher creates and stores a voucher for a newly initialized device in Passport
func (p *PassportVoucherState) NewVoucher(ctx context.Context, voucher *fdo.Voucher) error {
	if voucher == nil {
		return fmt.Errorf("voucher cannot be nil")
	}

	// Convert FDO voucher to Passport format
	// Use device info as deployed location, or default
	deployedLocation := voucher.Header.Val.DeviceInfo
	passportData, err := p.converter.VoucherToPassport(voucher, deployedLocation)
	if err != nil {
		return fmt.Errorf("error converting voucher to passport format: %w", err)
	}

	// Store in Passport API
	if err := p.client.StorePassportCommissioning(ctx, passportData); err != nil {
		return fmt.Errorf("error storing commissioning data in passport: %w", err)
	}

	// Cache the voucher for local access
	guid := fmt.Sprintf("%x", voucher.Header.Val.GUID[:])
	p.voucherCache[guid] = voucher

	return nil
}

// Implementation of OwnerVoucherPersistentState interface

// AddVoucher stores the voucher of a device owned by the service in Passport
func (p *PassportVoucherState) AddVoucher(ctx context.Context, voucher *fdo.Voucher) error {
	return p.NewVoucher(ctx, voucher)
}

// ReplaceVoucher stores a new voucher, replacing the previous one in Passport
func (p *PassportVoucherState) ReplaceVoucher(ctx context.Context, guid protocol.GUID, voucher *fdo.Voucher) error {
	if voucher == nil {
		return fmt.Errorf("voucher cannot be nil")
	}

	// Convert FDO voucher to Passport format
	deployedLocation := voucher.Header.Val.DeviceInfo
	passportData, err := p.converter.VoucherToPassport(voucher, deployedLocation)
	if err != nil {
		return fmt.Errorf("error converting voucher to passport format: %w", err)
	}

	// Store in Passport API (this will replace the existing one)
	if err := p.client.StorePassportCommissioning(ctx, passportData); err != nil {
		return fmt.Errorf("error storing commissioning data in passport: %w", err)
	}

	// Update cache
	guidStr := fmt.Sprintf("%x", guid[:])
	p.voucherCache[guidStr] = voucher

	return nil
}

// RemoveVoucher untracks a voucher from Passport and returns it for extension
func (p *PassportVoucherState) RemoveVoucher(ctx context.Context, guid protocol.GUID) (*fdo.Voucher, error) {
	guidStr := fmt.Sprintf("%x", guid[:])

	// Get the voucher first
	voucher, err := p.Voucher(ctx, guid)
	if err != nil {
		return nil, err
	}

	// In a real implementation, you might call a delete API on Passport
	// For now, we'll just remove from cache and return the voucher
	delete(p.voucherCache, guidStr)

	return voucher, nil
}

// Voucher retrieves a voucher by GUID from Passport or cache
func (p *PassportVoucherState) Voucher(ctx context.Context, guid protocol.GUID) (*fdo.Voucher, error) {
	guidStr := fmt.Sprintf("%x", guid[:])

	// Check cache first
	if voucher, exists := p.voucherCache[guidStr]; exists {
		return voucher, nil
	}

	// Try to get from Passport API
	_, err := p.client.GetPassportCommissioning(ctx, guidStr)
	if err != nil {
		return nil, fmt.Errorf("error getting commissioning data from passport: %w", err)
	}

	// For now, we can't convert back from Passport to voucher fully
	// In a real implementation, you'd need to either:
	// 1. Store the original voucher alongside passport data
	// 2. Implement full reverse conversion
	// 3. Keep vouchers in cache during protocol execution
	return nil, fmt.Errorf("voucher reconstruction from passport not yet implemented - GUID: %s", guidStr)
}

// Implementation of OwnerVoucherPersistentState interface for FallbackVoucherState

// NewVoucher delegates to primary state
func (f *FallbackVoucherState) NewVoucher(ctx context.Context, voucher *fdo.Voucher) error {
	return f.primaryState.NewVoucher(ctx, voucher)
}

// AddVoucher delegates to primary state
func (f *FallbackVoucherState) AddVoucher(ctx context.Context, voucher *fdo.Voucher) error {
	return f.primaryState.AddVoucher(ctx, voucher)
}

// ReplaceVoucher delegates to primary state
func (f *FallbackVoucherState) ReplaceVoucher(ctx context.Context, guid protocol.GUID, voucher *fdo.Voucher) error {
	return f.primaryState.ReplaceVoucher(ctx, guid, voucher)
}

// RemoveVoucher delegates to primary state
func (f *FallbackVoucherState) RemoveVoucher(ctx context.Context, guid protocol.GUID) (*fdo.Voucher, error) {
	return f.primaryState.RemoveVoucher(ctx, guid)
}

// Voucher implements the fallback mechanism:
// 1. First tries to get voucher from conversion layer (primary state)
// 2. If that fails, falls back to calling Passport directly during TO2
func (f *FallbackVoucherState) Voucher(ctx context.Context, guid protocol.GUID) (*fdo.Voucher, error) {
	// First, try the primary state (conversion layer)
	voucher, err := f.primaryState.Voucher(ctx, guid)
	if err == nil {
		// Success! Return the voucher from conversion layer
		return voucher, nil
	}

	// Conversion layer failed, fall back to direct Passport call during TO2
	// This is the key fallback mechanism
	guidStr := fmt.Sprintf("%x", guid[:])
	
	// Try to get commissioning data directly from Passport using the product item endpoint
	// This aligns with the user-provided API structure: GET /product_item/?uuid=<uuid>
	url := fmt.Sprintf("%s/product_item/?uuid=%s", f.client.Config().BaseURL, guidStr)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for product item: %w", err)
	}
	
	resp, err := f.client.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("both conversion layer and direct Passport call failed - conversion error: %v, passport error: %w", 
			err, err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("both conversion layer and direct Passport call failed - conversion error: %v, passport error: status %d", 
			err, resp.StatusCode)
	}
	
	// Parse the PassportProductItemResponse directly
	var productItemResponse PassportProductItemResponse
	if err := json.NewDecoder(resp.Body).Decode(&productItemResponse); err != nil {
		return nil, fmt.Errorf("conversion layer failed, direct Passport call succeeded but response parsing failed: %w", err)
	}
	
	// Use the new method to create voucher from product item response
	voucher, err = f.converter.CreateVoucherFromProductItemResponse(&productItemResponse)
	if err != nil {
		return nil, fmt.Errorf("conversion layer failed, direct Passport call succeeded but voucher creation failed: %w", err)
	}

	// Cache the converted voucher for future use
	f.primaryState.voucherCache[guidStr] = voucher

	return voucher, nil
}

// VoucherStateInterface defines the interface for voucher state operations
type VoucherStateInterface interface {
	NewVoucher(context.Context, *fdo.Voucher) error
	AddVoucher(context.Context, *fdo.Voucher) error
	ReplaceVoucher(context.Context, protocol.GUID, *fdo.Voucher) error
	RemoveVoucher(context.Context, protocol.GUID) (*fdo.Voucher, error)
	Voucher(context.Context, protocol.GUID) (*fdo.Voucher, error)
}

// PassportIntegratedServer wraps Passport integration functionality
type PassportIntegratedServer struct {
	passportState VoucherStateInterface
	client        *PassportClient
	converter     *VoucherToPassportConverter
}

// NewPassportIntegratedServer creates a new FDO server integration with Passport
func NewPassportIntegratedServer(passportClient *PassportClient) *PassportIntegratedServer {
	passportState := NewPassportVoucherState(passportClient)
	converter := NewVoucherToPassportConverter(passportClient)

	return &PassportIntegratedServer{
		passportState: passportState,
		client:        passportClient,
		converter:     converter,
	}
}

// NewFallbackPassportIntegratedServer creates a new FDO server integration with Passport
// that includes the fallback mechanism for TO2.
func NewFallbackPassportIntegratedServer(passportClient *PassportClient) *PassportIntegratedServer {
	passportState := NewFallbackVoucherState(passportClient)
	converter := NewVoucherToPassportConverter(passportClient)

	return &PassportIntegratedServer{
		passportState: passportState,
		client:        passportClient,
		converter:     converter,
	}
}

// GetVoucherState returns the Passport voucher state that can be used with FDO servers
func (p *PassportIntegratedServer) GetVoucherState() VoucherStateInterface {
	return p.passportState
}

// OnVoucherCreated is a callback that gets called when a new voucher is created
// It immediately syncs the voucher to Passport
func (p *PassportIntegratedServer) OnVoucherCreated(ctx context.Context, voucher *fdo.Voucher) error {
	// Convert and store in Passport
	deployedLocation := voucher.Header.Val.DeviceInfo
	passportData, err := p.converter.VoucherToPassport(voucher, deployedLocation)
	if err != nil {
		return fmt.Errorf("error converting voucher to passport format: %w", err)
	}

	if err := p.client.StorePassportCommissioning(ctx, passportData); err != nil {
		return fmt.Errorf("error storing commissioning data in passport: %w", err)
	}

	return nil
}

// OnVoucherExtended is a callback that gets called when a voucher is extended
// It updates the corresponding Passport commissioning data
func (p *PassportIntegratedServer) OnVoucherExtended(ctx context.Context, oldVoucher, newVoucher *fdo.Voucher) error {
	// Convert and update in Passport
	deployedLocation := newVoucher.Header.Val.DeviceInfo
	passportData, err := p.converter.VoucherToPassport(newVoucher, deployedLocation)
	if err != nil {
		return fmt.Errorf("error converting extended voucher to passport format: %w", err)
	}

	if err := p.client.StorePassportCommissioning(ctx, passportData); err != nil {
		return fmt.Errorf("error updating commissioning data in passport: %w", err)
	}

	return nil
}
