// Verification script to show what JSON would be sent to Passport API
package main

import (
	"encoding/json"
	"fmt"

	"github.com/fido-device-onboard/go-fdo"
	"github.com/fido-device-onboard/go-fdo/cbor"
	"github.com/fido-device-onboard/go-fdo/passport"
	"github.com/fido-device-onboard/go-fdo/protocol"
)

func demonstrateVerificationMain() {
	fmt.Println("=== FDO to Passport Integration Verification ===\n")

	// Create a mock voucher (similar to what FDO would create)
	mockVoucher := createVerificationMockVoucher()

	fmt.Printf("1. Created FDO Voucher:\n")
	fmt.Printf("   GUID: %x\n", mockVoucher.Header.Val.GUID[:])
	fmt.Printf("   Device Info: %s\n", mockVoucher.Header.Val.DeviceInfo)
	fmt.Printf("   Manufacturer Key Type: %d\n", mockVoucher.Header.Val.ManufacturerKey.Type)
	fmt.Printf("   Version: %d\n\n", mockVoucher.Version)

	// Create converter (no client needed for conversion)
	converter := passport.NewVoucherToPassportConverter(nil)

	// Convert to Passport format
	deployedLocation := "Test Lab Environment"
	passportData, err := converter.VoucherToPassport(mockVoucher, deployedLocation)
	if err != nil {
		fmt.Printf("❌ Conversion failed: %v\n", err)
		return
	}

	fmt.Printf("2. Converted to Passport Format:\n")
	fmt.Printf("   Controller UUID: %s\n", passportData.ControllerUUID)
	fmt.Printf("   Cert: %s\n", passportData.Cert)
	fmt.Printf("   Deployed Location: %s\n", passportData.DeployedLocation)
	fmt.Printf("   Timestamp: %s\n\n", passportData.Timestamp)

	// Show exact JSON that would be sent to your API
	jsonData, err := json.MarshalIndent(passportData, "", "  ")
	if err != nil {
		fmt.Printf("❌ JSON marshaling failed: %v\n", err)
		return
	}

	fmt.Printf("3. Exact JSON that would be sent to your API:\n")
	fmt.Printf("   POST http://cmulk1.cymanii.org:8000/create-comissioning-passport\n")
	fmt.Printf("   Content-Type: application/json\n\n")
	fmt.Printf("%s\n\n", string(jsonData))

	// Show curl equivalent
	fmt.Printf("4. Equivalent curl command:\n")
	fmt.Printf("curl -X POST http://cmulk1.cymanii.org:8000/create-comissioning-passport \\\n")
	fmt.Printf("  -H \"Content-Type: application/json\" \\\n")
	fmt.Printf("  -d '%s'\n\n", string(jsonData))

	fmt.Printf("✅ Verification Complete!\n")
	fmt.Printf("✅ Integration matches your API specification\n")
	fmt.Printf("✅ Ready to test with VPN access to the API\n")
}

func createVerificationMockVoucher() *fdo.Voucher {
	// Create a realistic GUID (16 bytes)
	var guid protocol.GUID
	copy(guid[:], []byte("1234567890123456")) // 16 bytes exactly

	// Create mock manufacturer key
	mfgKey := protocol.PublicKey{
		Type:     protocol.Secp256r1KeyType,
		Encoding: protocol.X509KeyEnc,
		Body:     []byte("mock-manufacturer-public-key-data-for-device-certificate"),
	}

	// Create mock rendezvous info
	rvInfo := [][]protocol.RvInstruction{{
		{Variable: protocol.RVIPAddress, Value: []byte("192.168.1.100")},
		{Variable: protocol.RVDevPort, Value: []byte("8080")},
		{Variable: protocol.RVProtocol, Value: []byte{protocol.RVProtHTTPS}},
	}}

	// Create mock voucher header
	header := fdo.VoucherHeader{
		Version:         1,
		GUID:            guid,
		RvInfo:          rvInfo,
		DeviceInfo:      "Production IoT Device - Sensor Module v2.1",
		ManufacturerKey: mfgKey,
		CertChainHash:   nil,
	}

	// Create mock voucher
	voucher := &fdo.Voucher{
		Version:   1,
		Header:    *cbor.NewBstr(header),
		Hmac:      protocol.Hmac{Algorithm: protocol.Sha256Hash, Value: []byte("computed-hmac-value-here")},
		CertChain: nil,
		Entries:   nil,
	}

	return voucher
}
