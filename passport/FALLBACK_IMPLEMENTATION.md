# Fallback Mechanism Implementation for TO2

This document explains the implementation of the fallback mechanism that allows the FDO library to fall back to calling Passport directly during TO2 if the conversion layer doesn't work.

## Overview

The fallback mechanism ensures that:
1. **Primary approach**: The conversion layer attempts to retrieve vouchers from Passport and convert them to FDO format
2. **Fallback approach**: If the conversion layer fails, the system falls back to calling Passport directly during TO2
3. **Voucher system preservation**: The existing FDO voucher system remains completely intact
4. **Seamless integration**: The fallback is transparent to the calling code

## Architecture

### Components

1. **`FallbackVoucherState`**: Implements the `OwnerVoucherPersistentState` interface with fallback logic
2. **`VoucherStateInterface`**: Common interface for voucher state operations
3. **`PassportIntegratedServer`**: Server integration that can use either standard or fallback voucher state

### Flow Diagram

```
TO2.ProveOVHdr Request
         ↓
   Voucher Retrieval
         ↓
┌─────────────────────────────────┐
│ 1. Try Conversion Layer        │
│    - Check cache               │
│    - Convert Passport → Voucher│
└─────────────────────────────────┘
         ↓
   Success? ──Yes──→ Return Voucher
         ↓ No
┌─────────────────────────────────┐
│ 2. Fallback to Direct Passport │
│    - Call Passport API directly│
│    - Convert response to voucher│
│    - Cache for future use      │
└─────────────────────────────────┘
         ↓
   Return Converted Voucher
```

## Usage

### Basic Fallback Setup

```go
// Create Passport client
config := &passport.PassportConfig{
    BaseURL: "https://passport-api.example.com",
    APIKey:  "your-api-key",
}
client := passport.NewPassportClient(config)

// Create fallback voucher state
fallbackState := passport.NewFallbackVoucherState(client)

// Use in your FDO server
server := &fdo.TO2Server{
    Vouchers: fallbackState,
    // ... other fields
}
```

### Integrated Server with Fallback

```go
// Create integrated server with fallback capability
integratedServer := passport.NewFallbackPassportIntegratedServer(client)

// Get the voucher state for your server
voucherState := integratedServer.GetVoucherState()

// Use in your FDO server
server := &fdo.TO2Server{
    Vouchers: voucherState,
    // ... other fields
}
```

## Implementation Details

### Fallback Logic

The fallback mechanism is implemented in the `Voucher` method of `FallbackVoucherState`:

```go
func (f *FallbackVoucherState) Voucher(ctx context.Context, guid protocol.GUID) (*fdo.Voucher, error) {
    // 1. First, try the primary state (conversion layer)
    voucher, err := f.primaryState.Voucher(ctx, guid)
    if err == nil {
        // Success! Return the voucher from conversion layer
        return voucher, nil
    }

    // 2. Conversion layer failed, fall back to direct Passport call during TO2
    // This aligns with the user-provided API structure: GET /product_item/?uuid=<uuid>
    guidStr := fmt.Sprintf("%x", guid[:])
    
    // Try to get commissioning data directly from Passport using the product item endpoint
    url := fmt.Sprintf("%s/product_item/?uuid=%s", f.client.Config().BaseURL, guidStr)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("error creating request for product item: %w", err)
    }
    
    resp, err := f.client.Config().Client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("both conversion layer and direct Passport call failed")
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("both conversion layer and direct Passport call failed - status %d", resp.StatusCode)
    }
    
    // Parse the PassportProductItemResponse directly
    var productItemResponse PassportProductItemResponse
    if err := json.NewDecoder(resp.Body).Decode(&productItemResponse); err != nil {
        return nil, fmt.Errorf("conversion layer failed, direct Passport call succeeded but response parsing failed: %w", err)
    }
    
    // 3. Use the new method to create voucher from product item response
    voucher, err = f.converter.CreateVoucherFromProductItemResponse(&productItemResponse)
    if err != nil {
        return nil, fmt.Errorf("voucher creation failed")
    }

    // 4. Cache the converted voucher for future use
    f.primaryState.voucherCache[guidStr] = voucher

    return voucher, nil
}
```

### Integration Points

The fallback mechanism integrates with TO2 at these key points:

1. **`TO2Server.proveOVHdr`**: Calls `s.Vouchers.Voucher(ctx, hello.GUID)`
2. **`TO2Server.ovNextEntry`**: Calls `s.Vouchers.Voucher(ctx, guid)`
3. **`TO2Server.setupDevice`**: Calls `s.Vouchers.Voucher(ctx, guid)`
4. **`TO2Server.ownerServiceInfo`**: Calls `s.Vouchers.Voucher(ctx, guid)`
5. **`TO2Server.to2Done2`**: Calls `s.Vouchers.Voucher(ctx, currentGUID)`

## Benefits

1. **Reliability**: Ensures TO2 can proceed even if the conversion layer has issues
2. **Performance**: Caches converted vouchers to avoid repeated API calls
3. **Transparency**: No changes required to existing FDO code
4. **Flexibility**: Can be enabled/disabled by choosing different voucher state implementations

## Error Handling

The fallback mechanism provides detailed error information:

- **Primary failure**: Logs the conversion layer error
- **Fallback failure**: Logs both conversion and Passport API errors
- **Conversion failure**: Logs the specific conversion error after successful API call

## Testing

Use the provided example to test the fallback mechanism:

```bash
cd passport/example
go run fallback_example.go
```

## API Alignment

The fallback mechanism is aligned with the user-provided Passport API endpoints:

### Endpoints Used

1. **`POST /create-comissioning-passport`**: Used by the conversion layer to store commissioning data
2. **`GET /product_item/?uuid=<uuid>`**: Used by the fallback mechanism to retrieve commissioning data directly

### Response Structure

The fallback mechanism expects the `GET /product_item` response to have this structure:
```json
{
  "schema_version": 1.0,
  "uuid": "191e886b-dfff-4f39-9618-d7a364ec0c90",
  "records": [
    {
      "uuid": "record-uuid",
      "signature": "certificate-data",
      "descriptor": "PRODUCT PASSPORT"
    }
  ]
}
```

The fallback mechanism specifically looks for records with `"descriptor": "PRODUCT PASSPORT"` to extract commissioning data.

## Configuration

The fallback mechanism requires:

1. **Passport API access**: Valid API endpoint and credentials
2. **Network connectivity**: Access to Passport API during TO2
3. **Timeout configuration**: Appropriate timeouts for API calls

## Security Considerations

1. **API Key Management**: Ensure Passport API keys are securely stored
2. **Network Security**: Use HTTPS for all Passport API communications
3. **Rate Limiting**: Be aware of Passport API rate limits during TO2
4. **Error Logging**: Avoid logging sensitive information in error messages

## Troubleshooting

### Common Issues

1. **Conversion Layer Always Fails**: Check Passport API connectivity and credentials
2. **Fallback Never Triggers**: Verify the `FallbackVoucherState` is properly configured
3. **Performance Issues**: Check caching behavior and API response times

### Debug Mode

Enable debug logging to see the fallback mechanism in action:

```go
// Set log level to debug to see fallback decisions
log.SetLevel(log.DebugLevel)
```

## Future Enhancements

1. **Retry Logic**: Add exponential backoff for failed API calls
2. **Circuit Breaker**: Implement circuit breaker pattern for API failures
3. **Metrics**: Add metrics for fallback usage and performance
4. **Configuration**: Make fallback behavior configurable per environment
