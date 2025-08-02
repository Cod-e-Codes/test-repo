package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Cod-e-Codes/marchat/plugin/license"
)

func main() {
	var (
		action      = flag.String("action", "", "Action to perform: validate, generate, or genkey")
		licenseFile = flag.String("license", "", "License file path")
		pluginName  = flag.String("plugin", "", "Plugin name")
		customerID  = flag.String("customer", "", "Customer ID")
		expiresAt   = flag.String("expires", "", "Expiration date (YYYY-MM-DD)")
		privateKey  = flag.String("private-key", "", "Private key for signing")
		publicKey   = flag.String("public-key", "", "Public key for validation")
		cacheDir    = flag.String("cache-dir", "./license-cache", "License cache directory")
		outputFile  = flag.String("output", "", "Output file for generated license")
	)
	flag.Parse()

	switch *action {
	case "validate":
		if *licenseFile == "" {
			fmt.Println("Error: license file path required")
			flag.Usage()
			os.Exit(1)
		}
		if *publicKey == "" {
			fmt.Println("Error: public key required for validation")
			flag.Usage()
			os.Exit(1)
		}
		validateLicense(*licenseFile, *publicKey, *cacheDir)

	case "generate":
		if *pluginName == "" || *customerID == "" || *expiresAt == "" || *privateKey == "" {
			fmt.Println("Error: plugin name, customer ID, expiration date, and private key required")
			flag.Usage()
			os.Exit(1)
		}
		generateLicense(*pluginName, *customerID, *expiresAt, *privateKey, *outputFile)

	case "genkey":
		generateKeyPair()

	case "check":
		if *pluginName == "" {
			fmt.Println("Error: plugin name required")
			flag.Usage()
			os.Exit(1)
		}
		if *publicKey == "" {
			fmt.Println("Error: public key required for validation")
			flag.Usage()
			os.Exit(1)
		}
		checkLicense(*pluginName, *publicKey, *cacheDir)

	default:
		fmt.Println("Error: action required")
		flag.Usage()
		os.Exit(1)
	}
}

func validateLicense(licensePath, publicKeyBase64, cacheDir string) {
	validator, err := license.NewLicenseValidator(publicKeyBase64, cacheDir)
	if err != nil {
		fmt.Printf("Error creating validator: %v\n", err)
		os.Exit(1)
	}

	license, err := validator.ValidateLicense(licensePath)
	if err != nil {
		fmt.Printf("License validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("License validated successfully!\n")
	fmt.Printf("Plugin: %s\n", license.PluginName)
	fmt.Printf("Customer: %s\n", license.CustomerID)
	fmt.Printf("Issued: %s\n", license.IssuedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Expires: %s\n", license.ExpiresAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Features: %v\n", license.Features)
	if license.MaxUsers > 0 {
		fmt.Printf("Max Users: %d\n", license.MaxUsers)
	}
}

func generateLicense(pluginName, customerID, expiresAtStr, privateKeyBase64, outputFile string) {
	// Parse expiration date
	expiresAt, err := time.Parse("2006-01-02", expiresAtStr)
	if err != nil {
		fmt.Printf("Error parsing expiration date: %v\n", err)
		os.Exit(1)
	}

	license, err := license.GenerateLicense(pluginName, customerID, expiresAt, privateKeyBase64)
	if err != nil {
		fmt.Printf("Error generating license: %v\n", err)
		os.Exit(1)
	}

	// Marshal license to JSON
	data, err := json.MarshalIndent(license, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling license: %v\n", err)
		os.Exit(1)
	}

	// Write to file or stdout
	if outputFile != "" {
		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			fmt.Printf("Error writing license file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("License written to %s\n", outputFile)
	} else {
		fmt.Println(string(data))
	}
}

func generateKeyPair() {
	publicKey, privateKey, err := license.GenerateKeyPair()
	if err != nil {
		fmt.Printf("Error generating key pair: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generated key pair:")
	fmt.Printf("Public Key: %s\n", publicKey)
	fmt.Printf("Private Key: %s\n", privateKey)
	fmt.Println("\nStore these keys securely!")
}

func checkLicense(pluginName, publicKeyBase64, cacheDir string) {
	validator, err := license.NewLicenseValidator(publicKeyBase64, cacheDir)
	if err != nil {
		fmt.Printf("Error creating validator: %v\n", err)
		os.Exit(1)
	}

	valid, err := validator.IsLicenseValid(pluginName)
	if err != nil {
		fmt.Printf("Error checking license: %v\n", err)
		os.Exit(1)
	}

	if valid {
		fmt.Printf("License for plugin %s is valid\n", pluginName)
	} else {
		fmt.Printf("No valid license found for plugin %s\n", pluginName)
	}
}
