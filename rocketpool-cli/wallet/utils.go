package wallet

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rocket-pool/rocketpool-go/types"
	"github.com/rocket-pool/smartnode/rocketpool-cli/wallet/bip39"
	"github.com/rocket-pool/smartnode/shared/services/passwords"
	"github.com/rocket-pool/smartnode/shared/services/rocketpool"
	"github.com/rocket-pool/smartnode/shared/types/api"
	cliutils "github.com/rocket-pool/smartnode/shared/utils/cli"
	hexutils "github.com/rocket-pool/smartnode/shared/utils/hex"
)

const (
	passwordKeyFormat string = "PASSWORD_%s"
)

// Prompt for a wallet password
func promptPassword() string {
	for {
		password := cliutils.PromptPassword(
			"Please enter a password to secure your wallet with:",
			fmt.Sprintf("^.{%d,}$", passwords.MinPasswordLength),
			fmt.Sprintf("Your password must be at least %d characters long. Please try again:", passwords.MinPasswordLength),
		)
		confirmation := cliutils.PromptPassword("Please confirm your password:", "^.*$", "")
		if password == confirmation {
			return password
		} else {
			fmt.Println("Password confirmation does not match.")
			fmt.Println("")
		}
	}
}

// Prompt for a recovery mnemonic phrase
func promptMnemonic() string {
	for {
		lengthInput := cliutils.Prompt(
			"Please enter the number of words in your mnemonic phrase (24 by default):",
			"^[1-9][0-9]*$",
			"Please enter a valid number.")

		length, err := strconv.Atoi(lengthInput)
		if err != nil {
			fmt.Println("Please enter a valid number.")
			continue
		}

		mv := bip39.Create(length)
		if mv == nil {
			fmt.Println("Please enter a valid mnemonic length.")
			continue
		}

		i := 0
		for mv.Filled() == false {
			prompt := fmt.Sprintf("Enter Word Number %d of your mnemonic:", i+1)
			word := cliutils.PromptPassword(prompt, "^[a-zA-Z]+$", "Please enter a single word only.")

			if err := mv.AddWord(strings.ToLower(word)); err != nil {
				fmt.Println("Inputted word not valid, please retry.")
				continue
			}

			i++
		}

		mnemonic, err := mv.Finalize()
		if err != nil {
			fmt.Printf("Error validating mnemonic: %s\n", err)
			fmt.Println("Please try again.")
			fmt.Println("")
			continue
		}

		return mnemonic
	}
}

// Confirm a recovery mnemonic phrase
func confirmMnemonic(mnemonic string) {
	for {
		fmt.Println("Please enter your mnemonic phrase to confirm.")
		confirmation := promptMnemonic()
		if mnemonic == confirmation {
			return
		} else {
			fmt.Println("The mnemonic phrase you entered does not match your recovery phrase. Please try again.")
			fmt.Println("")
		}
	}
}

// Check for custom keys and prompt for their passwords
func promptForCustomKeyPasswords(rp *rocketpool.Client) (map[string]string, error) {

	// Load the config
	cfg, _, err := rp.LoadConfig()
	if err != nil {
		return nil, err
	}

	// Check for the custom key directory
	customKeyDir := filepath.Join(cfg.Smartnode.DataPath.Value.(string), "custom-keys")
	info, err := os.Stat(customKeyDir)
	if os.IsNotExist(err) || !info.IsDir() {
		return nil, nil
	}

	// Get the custom keystore files
	files, err := ioutil.ReadDir(customKeyDir)
	if err != nil {
		return nil, fmt.Errorf("error enumerating custom keystores: %w", err)
	}
	if len(files) == 0 {
		return nil, nil
	}

	// Get the pubkeys for the custom keystores
	customPubkeys := []types.ValidatorPubkey{}
	for _, file := range files {
		// Read the file
		bytes, err := ioutil.ReadFile(filepath.Join(customKeyDir, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("error reading custom keystore %s: %w", file.Name(), err)
		}

		// Deserialize it
		keystore := api.ValidatorKeystore{}
		err = json.Unmarshal(bytes, &keystore)
		if err != nil {
			return nil, fmt.Errorf("error deserializing custom keystore %s: %w", file.Name(), err)
		}

		customPubkeys = append(customPubkeys, keystore.Pubkey)
	}

	// Notify the user
	fmt.Println("It looks like you have some custom keystores for your minipool's validators.\nYou will be prompted for the passwords each one was encrypted with, so they can be loaded into the Validator Client that Rocket Pool manages for you.\n")

	// Get the passwords for each one
	pubkeyPasswords := map[string]string{}
	for _, pubkey := range customPubkeys {
		password := cliutils.PromptPassword(
			fmt.Sprintf("Please enter a password that the keystore for %s was encrypted with:", pubkey.Hex()), "^.*$", "",
		)

		formattedPubkey := strings.ToUpper(hexutils.RemovePrefix(pubkey.Hex()))
		pubkeyPasswords[fmt.Sprintf(passwordKeyFormat, formattedPubkey)] = password

		fmt.Println()
	}
	return pubkeyPasswords, nil

}
