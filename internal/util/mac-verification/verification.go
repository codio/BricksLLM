package mac_verification

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/google/tink/go/insecurecleartextkeyset"
	"github.com/google/tink/go/keyset"
	"github.com/google/tink/go/mac"
	"github.com/google/tink/go/tink"
)

const keySetSubDir = "./tink"

var macCache map[string]tink.MAC = make(map[string]tink.MAC)

func init() {
	err := loadKeySets()
	if err != nil {
		log.Fatal(err)
	}
}

func loadKeySets() error {
	execDir, err := os.Executable()
	if err != nil {
		return fmt.Errorf("error getting executable path: %v", err)
	}
	execDir = filepath.Dir(execDir)
	keySetDir := filepath.Join(execDir, keySetSubDir)
	files, err := os.ReadDir(keySetDir)
	if err != nil {
		return fmt.Errorf("error reading directory %s: %v", keySetDir, err)
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filePath := filepath.Join(keySetDir, file.Name())
		f, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("error opening keyset file %s: %v", filePath, err)
		}
		defer f.Close()

		jsonReader := keyset.NewJSONReader(f)

		kh, err := insecurecleartextkeyset.Read(jsonReader)
		if err != nil {
			return fmt.Errorf("error reading keyset from file %s: %v", filePath, err)
		}

		macPrimitive, err := mac.New(kh)
		if err != nil {
			return fmt.Errorf("error creating MAC primitive from keyset %s: %v", filePath, err)
		}

		providerName := file.Name()
		if len(providerName) > 5 && providerName[len(providerName)-5:] == ".json" {
			providerName = providerName[:len(providerName)-5]
		}
		macCache[providerName] = macPrimitive
	}
	return nil
}

func VerifySign(provider string, data []byte, token string) bool {
	decodedSignature, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		fmt.Printf("Error decoding token from Base64: %v\n", err)
		return false
	}
	macPrimitive, ok := macCache[provider]
	if !ok {
		fmt.Printf("MAC primitive for provider %s not found\n", provider)
		return false
	}
	err = macPrimitive.VerifyMAC(decodedSignature, data)
	if err != nil {
		return false
	}
	return true
}
