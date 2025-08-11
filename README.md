# go-fdo-passport
Integration of Passport API with the open-source FDO library. Added a conversion layer to transform Passport API data into vouchers, retained original voucher system, and implemented a fallback mechanism for TO2. Includes examples, verification scripts, and clean integration code
# 🚀 FDO Passport API Integration with Smart Fallback Mechanism

[![Go Version](https://img.shields.io/badge/go-%3E%3D%201.23-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![FDO Spec](https://img.shields.io/badge/FDO-Specification%201.1-green.svg)](https://fidoalliance.org/specs/FDO/)

> **Seamlessly integrate FIDO Device Onboard (FDO) with Passport API while maintaining 100% compatibility with existing systems.**

## 🎯 **What This Project Delivers**

This project implements a **comprehensive FDO Passport integration** that meets four critical requirements:

1. ✅ **Keep the voucher system in place** - No changes to core FDO functionality
2. ✅ **Add conversion layer** - Pulls data from Passport API and converts to voucher format
3. ✅ **Do not replace voucher completely** - Hybrid approach with Passport as intelligent backend
4. ✅ **Smart fallback mechanism** - Only calls Passport during TO2 when conversion layer fails

## �� **Key Features**

- **🔄 Smart Fallback System** - Automatically switches between conversion layer and direct API calls
- **🔒 Zero Breaking Changes** - Existing FDO servers work without modification
- **📡 Passport API Integration** - Connects to your Passport endpoints seamlessly
- **💾 Bidirectional Conversion** - FDO vouchers ↔ Passport commissioning data
- **⚡ TO2 Protocol Integration** - Seamless integration with device onboarding flow
- **🛡️ Production Ready** - Comprehensive testing and error handling

## 🏗️ **Architecture Overview**

```
FDO Server → PassportIntegratedServer → FallbackVoucherState → VoucherToPassportConverter → Passport API
                ↓
        VoucherStateInterface (common interface)
                ↓
        Primary Voucher State (conversion layer)
                ↓
        Fallback to Direct API Calls (when needed)
```

## 🔧 **Installation & Setup**

### Prerequisites
- Go 1.23 or higher
- Access to Passport API endpoints

### Quick Start
```bash
# Clone the repository
git clone https://github.com/your-username/go-fdo-passport.git
cd go-fdo-passport

# Initialize Go workspace
go work init
go work use -r .

# Build the project
go build ./passport
go build ./passport/example

# Run examples
go run ./passport/example
```

## 📡 **API Integration**

### Passport API Endpoints
- **`POST /create-comissioning-passport`** - Store device commissioning data
- **`GET /product_item/?uuid=<uuid>`** - Retrieve device data during fallback

### Example Usage
```go
// Configure Passport client
config := &passport.PassportConfig{
    BaseURL: "http://your-passport-api.com:8000",
    APIKey:  "", // No auth required per your spec
    Timeout: 30 * time.Second,
}

// Create Passport client
client := passport.NewPassportClient(config)

// Create integrated server with fallback capability
server := passport.NewFallbackPassportIntegratedServer(client)

// Use in your FDO server
fdoServer := &fdo.TO2Server{
    Vouchers: server.GetVoucherState(),
    // ... other configuration
}
```

## �� **How the Fallback Mechanism Works**

### 1. **Primary Attempt** (Conversion Layer)
- First tries to retrieve data through the conversion layer
- Uses cached Passport data if available
- Attempts to reconstruct voucher from existing Passport records

### 2. **Fallback Trigger** (Direct API Call)
- If conversion layer fails, automatically falls back to direct Passport API calls
- Calls `GET /product_item/?uuid=<uuid>` endpoint
- Converts the `PassportProductItemResponse` to FDO voucher format
- Caches the result for future use

### 3. **Integration Points**
- **TO2.ProveOVHdr**: Voucher retrieval with automatic fallback
- **TO2.ovNextEntry**: Ownership transfer with Passport data
- **TO2.setupDevice**: Device setup using Passport credentials
- **TO2.ownerServiceInfo**: Service information from Passport

## 📁 **Project Structure**

```
passport/
├── passport.go                    # Core Passport integration and conversion layer
├── voucher_state.go               # Fallback mechanism implementation
├── credential_loader.go           # Device credential loading from Passport
├── FALLBACK_IMPLEMENTATION.md    # Detailed technical documentation
├── fallback_test.go              # Unit tests for fallback mechanism
├── integration_test.go           # Integration tests for complete system
└── example/                      # Working demonstrations
    ├── main.go                   # Unified example runner with all demonstrations
    ├── device_initiation.go      # Device initiation with Passport integration
    ├── fallback_example.go       # Fallback mechanism demonstration
    ├── integration_example.go    # Complete integration demo
    └── verify.go                 # API verification and testing
```

## 🧪 **Testing**

```bash
# Run all tests
go test ./passport -v

# Run specific test suites
go test ./passport -run TestFallback
go test ./passport -run TestIntegration

# Build and test examples
go build ./passport/example
go run ./passport/example
```

## 📚 **Documentation**

- **[Main Integration Guide](passport/README.md)** - Detailed implementation guide
- **[Fallback Mechanism](passport/FALLBACK_IMPLEMENTATION.md)** - Technical deep-dive
- **[Examples](passport/example/)** - Working demonstrations
- **[API Reference](passport/passport.go)** - Go package documentation

## 🔐 **Security & Authentication**

- **No hardcoded secrets** - All sensitive data is configurable
- **API key support** - Optional authentication for Passport API
- **Secure defaults** - No authentication required by default (per your API spec)
- **Production ready** - Comprehensive error handling and validation

## 🚀 **Getting Started with Examples**

### Basic Integration
```go
// Create Passport-integrated FDO server
server := passport.NewPassportIntegratedServer(client)

// Handle device initialization
if err := handleDeviceInitialization(server); err != nil {
    log.Fatalf("Device initialization failed: %v", err)
}
```

### Fallback Mechanism
```go
// Create fallback voucher state
fallbackState := passport.NewFallbackVoucherState(client)

// This will automatically handle fallback during TO2
voucher, err := fallbackState.Voucher(ctx, deviceGUID)
if err != nil {
    // Fallback mechanism will have already tried direct API calls
    log.Printf("Voucher retrieval failed: %v", err)
}
```

### Voucher Conversion
```go
// Convert FDO voucher to Passport format
converter := passport.NewVoucherToPassportConverter(client)
passportData, err := converter.VoucherToPassport(voucher, "Production Lab")

// Store in Passport
err = client.StorePassportCommissioning(ctx, passportData)
```

## �� **Contributing**

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Setup
```bash
# Install development dependencies
go mod download

# Run linting
go vet ./...

# Run tests with coverage
go test ./passport -cover
```

## 📄 **License**

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🙏 **Acknowledgments**

- Built on the [FIDO Device Onboard Specification 1.1](https://fidoalliance.org/specs/FDO/)
- Implements the converter pattern as recommended by the FDO community
- Designed for seamless integration with existing FDO deployments

## 📞 **Support**

- **Issues**: [GitHub Issues](https://github.com/your-username/go-fdo-passport/issues)
- **Discussions**: [GitHub Discussions](https://github.com/your-username/go-fdo-passport/discussions)
- **Documentation**: [Project Wiki](https://github.com/your-username/go-fdo-passport/wiki)

---

**Ready to revolutionize your FDO deployment with Passport integration?** 🚀

[Get Started](#installation--setup) • [View Examples](passport/example/) • [Read Documentation](passport/README.md)

---

*Made with ❤️ for the FDO community*
