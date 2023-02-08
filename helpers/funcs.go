package helpers

import (
	"encoding/json"
	"io/ioutil"

	_ "github.com/go-sql-driver/mysql"
)

// GetConf - Reads the config file located
func GetConf(c *Config) error {

	jsonFile, err := ioutil.ReadFile("/etc/toku/config.json")
	if err != nil {
		//		log.Println("[GetAsteriskConfigs] Error getting asterisk AMI configs", err)
		return err
	}
	err = json.Unmarshal(jsonFile, &c)
	if err != nil {
		//		log.Println("[GetAsteriskConfigs] Error! Unmarshal", err)
		return err
	}
	return nil
}
