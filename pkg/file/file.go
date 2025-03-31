package file

import "os"

// writeConfigToFile writes the network configuration to the file
func WriteConfigToFile(filePath string, config string) error {

	// TODO: if file exist overwrite it, maybe force option needed or warning
	err := os.WriteFile(filePath, []byte(config), 0600)
	return err
}

func RemoveConfigFile(filePath string) error {

	// Check if the file exists before trying to delete it
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// File does not exist, nothing to delete
		return nil
	}

	// Delete the file
	err := os.Remove(filePath)
	if err != nil {
		return err
	}

	return nil
}
