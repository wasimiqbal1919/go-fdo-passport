// SPDX-FileCopyrightText: (C) 2024 Intel Corporation
// SPDX-License-Identifier: Apache 2.0

package passport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

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
// It wraps a primary voucher state and only falls back to direct Passport calls
// when the conversion layer fails AND the current operation is part of TO2 protocol.
type FallbackVoucherState struct {
	primaryState *PassportVoucherState
	client       *PassportClient
	converter    *VoucherToPassportConverter
	// Track if we're currently in TO2 protocol
	isTO2Active bool
	// Mutex to protect TO2 state
	to2Mutex sync.RWMutex
}

// NewFallbackVoucherState creates a new fallback voucher state that implements
// the fallback mechanism for TO2.
func NewFallbackVoucherState(client *PassportClient) *FallbackVoucherState {
	return &FallbackVoucherState{
		primaryState: NewPassportVoucherState(client),
		client:       client,
		converter:    NewVoucherToPassportConverter(client),
		isTO2Active:  false,
	}
}

// SetTO2Active marks that TO2 protocol is currently active
// This should be called by TO2 server when TO2 begins
func (f *FallbackVoucherState) SetTO2Active(active bool) {
	f.to2Mutex.Lock()
	defer f.to2Mutex.Unlock()
	f.isTO2Active = active
}

// IsTO2Active checks if TO2 protocol is currently active
func (f *FallbackVoucherState) IsTO2Active() bool {
	f.to2Mutex.RLock()
	defer f.to2Mutex.RUnlock()
	return f.isTO2Active
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

// Voucher implements the TO2-specific fallback mechanism:
// 1. First tries to get voucher from conversion layer (primary state)
// 2. If that fails AND we're in TO2 protocol, falls back to calling Passport directly
// 3. If not in TO2 protocol, returns the conversion layer error without fallback
func (f *FallbackVoucherState) Voucher(ctx context.Context, guid protocol.GUID) (*fdo.Voucher, error) {
	// First, try the primary state (conversion layer)
	voucher, err := f.primaryState.Voucher(ctx, guid)
	if err == nil {
		// Success! Return the voucher from conversion layer
		return voucher, nil
	}

	// Conversion layer failed - check if we're in TO2 protocol
	if !f.IsTO2Active() {
		// Not in TO2 protocol - return conversion layer error without fallback
		return nil, fmt.Errorf("conversion layer failed and fallback not allowed outside TO2 protocol: %w", err)
	}

	// We're in TO2 protocol - fallback to direct Passport call is allowed
	// This is the key fallback mechanism that only works during TO2
	guidStr := fmt.Sprintf("%x", guid[:])

	log.Printf("TO2 fallback: Conversion layer failed, falling back to direct Passport call for GUID: %s", guidStr)

	// Try to get commissioning data directly from Passport using the product item endpoint
	// This aligns with the user-provided API structure: GET /product_item/?uuid=<uuid>
	url := fmt.Sprintf("%s/product_item/?uuid=%s", f.client.Config().BaseURL, guidStr)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("TO2 fallback failed - error creating request for product item: %w", err)
	}

	resp, err := f.client.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TO2 fallback failed - both conversion layer and direct Passport call failed - conversion error: %v, passport error: %w",
			err, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TO2 fallback failed - both conversion layer and direct Passport call failed - conversion error: %v, passport error: status %d",
			err, resp.StatusCode)
	}

	// Parse the PassportProductItemResponse directly
	var productItemResponse PassportProductItemResponse
	if err := json.NewDecoder(resp.Body).Decode(&productItemResponse); err != nil {
		return nil, fmt.Errorf("TO2 fallback: conversion layer failed, direct Passport call succeeded but response parsing failed: %w", err)
	}

	// Use the new method to create voucher from product item response
	voucher, err = f.converter.CreateVoucherFromProductItemResponse(&productItemResponse)
	if err != nil {
		return nil, fmt.Errorf("TO2 fallback: conversion layer failed, direct Passport call succeeded but voucher creation failed: %w", err)
	}

	log.Printf("TO2 fallback: Successfully created voucher from Passport API for GUID: %s", guidStr)

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

// NewPassportIntegratedServer creates a new server with Passport integration
func NewPassportIntegratedServer(client *PassportClient) *PassportIntegratedServer {
	return &PassportIntegratedServer{
		passportState: NewPassportVoucherState(client),
		client:        client,
		converter:     NewVoucherToPassportConverter(client),
	}
}

// TO2ServerWrapper wraps the FDO TO2Server to properly manage TO2 state
// This ensures that fallback to Passport only happens during TO2 operations
type TO2ServerWrapper struct {
	*fdo.TO2Server
	fallbackState *FallbackVoucherState
}

// NewTO2ServerWrapper creates a new TO2 server wrapper that manages TO2 state
func NewTO2ServerWrapper(fallbackState *FallbackVoucherState) *TO2ServerWrapper {
	// Create the underlying TO2Server with the fallback state
	to2Server := &fdo.TO2Server{
		Vouchers: fallbackState,
		// Add other TO2Server fields as needed
	}

	return &TO2ServerWrapper{
		TO2Server:     to2Server,
		fallbackState: fallbackState,
	}
}

// StartTO2 marks the beginning of TO2 protocol and enables fallback
func (w *TO2ServerWrapper) StartTO2() {
	w.fallbackState.SetTO2Active(true)
	log.Printf("TO2 protocol started - fallback to Passport API is now enabled")
}

// EndTO2 marks the end of TO2 protocol and disables fallback
func (w *TO2ServerWrapper) EndTO2() {
	w.fallbackState.SetTO2Active(false)
	log.Printf("TO2 protocol ended - fallback to Passport API is now disabled")
}

// Respond overrides the TO2Server Respond method to manage TO2 state
// This is the main entry point for all TO2 operations
func (w *TO2ServerWrapper) Respond(ctx context.Context, msgType uint8, msg io.Reader) (respType uint8, resp any) {
	// Check if this is a TO2 message type
	if isTO2Message(msgType) {
		// Mark TO2 as active when any TO2 message is processed
		w.StartTO2()
		defer w.EndTO2() // Ensure TO2 state is cleaned up
	}

	// Call the underlying TO2Server method
	return w.TO2Server.Respond(ctx, msgType, msg)
}

// isTO2Message checks if the message type is a TO2 protocol message
func isTO2Message(msgType uint8) bool {
	// TO2 message types from protocol/message_types.go
	to2MessageTypes := map[uint8]bool{
		60: true, // TO2HelloDeviceMsgType
		61: true, // TO2ProveOVHdrMsgType
		62: true, // TO2GetOVNextEntryMsgType
		63: true, // TO2ProveDeviceMsgType
		64: true, // TO2SetupDeviceMsgType
		65: true, // TO2OwnerServiceInfoReadyMsgType
		66: true, // TO2OwnerServiceInfoMsgType
		67: true, // TO2DeviceServiceInfoReadyMsgType
		68: true, // TO2DeviceServiceInfoMsgType
		69: true, // TO2DoneMsgType
		70: true, // TO2Done2MsgType
	}

	return to2MessageTypes[msgType]
}
