# FDO Passport API Integration

This package provides integration between the FIDO Device Onboard (FDO) library and Passport API for device commissioning and management.

## Overview

The FDO Passport integration allows you to replace traditional FDO ownership vouchers with Passport-based commissioning data. This integration acts as a bridge between the FDO protocol and the Passport API, enabling seamless device management through Passport while maintaining compatibility with the standard FDO flow.

## Architecture

The integration follows the **converter pattern** approach recommended in the requirements:

- **VoucherToPassportConverter**: Converts FDO vouchers to Passport API format
- **PassportVoucherState**: Implements FDO voucher state interfaces using Passport as backend  
- **PassportClient**: Handles HTTP communication with Passport API
- **PassportIntegratedServer**: Provides callbacks for voucher lifecycle events

## Key Components

### PassportCommissioningData

Matches the actual Passport API specification:

```go
type PassportCommissioningData struct {
    ControllerUUID   string `json:"controller_uuid"`
    Cert             string `json:"cert"`  
    DeployedLocation string `json:"deployed_location"`
    Timestamp        string `json:"timestamp"`
}
```

This corresponds to the API call:
```bash
curl -X POST http://cmulk1.cymanii.org:8000/create-comissioning-passport \
-H "Content-Type: application/json" \
-d '{
       "controller_uuid": "191e886b-dfff-4f39-9618-d7a364ec0c90", 
       "cert": "string", 
       "deployed_location": "string",
       "timestamp": "1754509904342152960"
    }'
```

### VoucherToPassportConverter

Converts FDO vouchers to Passport format:

```go
converter := passport.NewVoucherToPassportConverter(client)
passportData, err := converter.VoucherToPassport(voucher, "Production Lab")
```

Mapping:
- `voucher.Header.Val.GUID` → `controller_uuid` (hex encoded)
- Certificate chain or manufacturer key → `cert` (hex encoded)  
- Device info or provided location → `deployed_location`
- Current timestamp → `timestamp` (nanoseconds since Unix epoch)

### PassportVoucherState

Implements FDO voucher persistence interfaces but uses Passport API as backend:

```go
// Implements both ManufacturerVoucherPersistentState and OwnerVoucherPersistentState
type PassportVoucherState struct {
    converter *VoucherToPassportConverter
    client    *PassportClient
    voucherCache map[string]*fdo.Voucher // Local cache for protocol execution
}
```

## Usage

### Basic Setup

```go
// Configure Passport API client
passportConfig := &passport.PassportConfig{
    BaseURL: "http://cmulk1.cymanii.org:8000",
    APIKey:  "", // No API key needed based on provided example
    Timeout: 30 * time.Second,
}

passportClient := passport.NewPassportClient(passportConfig)

// Create integration server
server := passport.NewPassportIntegratedServer(passportClient)
```

### Using with FDO Servers

```go
// Get the Passport-backed voucher state
voucherState := server.GetVoucherState()

// Use with DI Server (requires generic type parameter based on your device info type)
diServer := &fdo.DIServer[MyDeviceInfo]{
    Vouchers: voucherState,
    // ... other configuration
}

// Use with TO2 Server  
to2Server := &fdo.TO2Server{
    Vouchers: voucherState,
    // ... other configuration
}
```

### Manual Voucher Conversion

```go
converter := passport.NewVoucherToPassportConverter(passportClient)

// Convert voucher to Passport format
passportData, err := converter.VoucherToPassport(voucher, "Lab Environment")
if err != nil {
    return err
}

// Store in Passport
err = passportClient.StorePassportCommissioning(ctx, passportData)
if err != nil {
    return err
}

// Retrieve from Passport (for querying existing data)
retrieved, err := passportClient.GetPassportCommissioning(ctx, passportData.ControllerUUID)
```

### Event Callbacks

```go
// Called when a new voucher is created during DI
err := server.OnVoucherCreated(ctx, newVoucher)

// Called when a voucher is extended during ownership transfer
err := server.OnVoucherExtended(ctx, oldVoucher, newVoucher)
```

## API Endpoints

The integration uses these Passport API endpoints:

- **POST** `/create-comissioning-passport` - Store commissioning data
- **GET** `/product_item/?uuid={controller_uuid}` - Retrieve commissioning data

## Implementation Details

### Voucher Caching

The integration maintains a local cache of vouchers during protocol execution since:

1. Passport API doesn't store the complete FDO voucher structure
2. FDO protocols require access to cryptographic details not preserved in Passport
3. Reverse conversion from Passport to voucher is complex and not fully implemented

### Error Handling

The integration gracefully handles:
- Network connectivity issues with Passport API
- Missing commissioning data  
- API response errors
- Voucher validation failures

### Security Considerations

- Certificate data is hex-encoded for safe JSON transport
- GUID/Controller UUID provides unique device identification
- Local voucher cache is memory-only (not persistent)
- No sensitive cryptographic keys stored in Passport API

## Testing

Run the example to test the integration:

```bash
go run -tags="!linux" ./passport/example
```

This will:
1. Create a mock FDO voucher
2. Convert it to Passport format  
3. Attempt to store it via the Passport API
4. Demonstrate the conversion process

## Limitations

1. **Reverse Conversion**: PassportToVoucher is not fully implemented
2. **Certificate Handling**: Only basic certificate extraction is implemented
3. **Ownership Chain**: Complex ownership transfers may need additional mapping
4. **Cryptographic Verification**: Passport data can't be cryptographically verified like vouchers

## Future Enhancements

1. Implement full reverse conversion from Passport to voucher
2. Add support for complex ownership chains
3. Enhance certificate handling and validation
4. Add audit logging for voucher lifecycle events
5. Implement Passport data validation against FDO requirements

## Integration Approach

This implementation follows the **converter pattern** as suggested:

> "Most preferred approach is to build a converter that converts between passport and voucher. So the code still read and write to voucher but converter translate to passport."

The FDO library continues to work with vouchers internally, while the PassportVoucherState acts as an adapter that translates operations to Passport API calls. This minimizes changes to the core FDO logic while enabling Passport integration.

If challenges arise with this approach, the fallback option is available:

> "if we find a lot of challenges, we can keep the voucher as is, and at the end of protocol make a call to make commissioning passport."

The OnVoucherCreated and OnVoucherExtended callbacks support this fallback approach by allowing Passport commissioning calls at voucher lifecycle events.

## TO2-Specific Fallback Mechanism

The Passport integration now includes a **TO2-specific fallback mechanism** that fulfills the requirement: **"Only fall back to calling Passport during TO2 if conversion layer doesn't work."**

### How It Works

1. **Protocol-Aware Fallback**: The system tracks whether TO2 protocol is currently active
2. **Conditional Passport Calls**: Fallback to Passport API only happens during TO2 operations
3. **Automatic State Management**: TO2 state is automatically managed by the server wrapper
4. **Security Enforcement**: Passport API calls are blocked outside of TO2 protocol

### Key Components

#### `FallbackVoucherState`
- **TO2 State Tracking**: Uses `isTO2Active` flag with thread-safe mutex
- **Conditional Fallback**: Only allows Passport API calls when `IsTO2Active()` returns true
- **Protocol Isolation**: TO1, DI, and other protocols cannot trigger Passport fallback

#### `TO2ServerWrapper`
- **Automatic State Management**: Wraps the FDO TO2Server to manage TO2 state
- **Message Type Detection**: Identifies TO2 messages and enables fallback automatically
- **State Cleanup**: Ensures TO2 state is properly reset after each operation

### Usage Example

```go
// Create fallback voucher state
fallbackState := passport.NewFallbackVoucherState(client)

// Create TO2 server wrapper that manages TO2 state
to2Server := passport.NewTO2ServerWrapper(fallbackState)

// The server automatically:
// 1. Enables fallback when TO2 messages arrive
// 2. Disables fallback when TO2 messages complete
// 3. Only allows Passport API calls during TO2 operations
```

### Fallback Behavior

| Protocol | Fallback Allowed | Behavior |
|----------|------------------|----------|
| **TO1** | ❌ No | Returns conversion layer error without Passport fallback |
| **DI** | ❌ No | Returns conversion layer error without Passport fallback |
| **TO2** | ✅ Yes | Falls back to Passport API if conversion layer fails |
| **Other** | ❌ No | Returns conversion layer error without Passport fallback |

### Security Benefits

1. **Protocol Isolation**: Prevents unauthorized Passport API calls during non-TO2 operations
2. **Resource Protection**: Avoids unnecessary network calls and API usage
3. **Audit Trail**: Clear logging of when fallback is enabled/disabled
4. **Thread Safety**: Proper mutex protection for concurrent operations

### Implementation Details

The fallback mechanism integrates with TO2 at these key points:

1. **`TO2Server.Respond`**: Main entry point that detects TO2 messages
2. **Message Type Detection**: Uses predefined TO2 message type constants
3. **State Management**: Automatically enables/disables fallback based on protocol
4. **Error Handling**: Provides clear error messages for blocked fallback attempts

### Testing

Run the TO2-specific fallback example:

```bash
cd passport/example
go run to2_fallback_example.go
```

This demonstrates:
- Fallback state management
- Protocol-aware behavior
- Security enforcement
- Complete integration flow
